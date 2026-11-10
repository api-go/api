package api

import (
	"context"
	"errors"
	"sync"

	"github.com/api-go/safe"
)

// Signer 负责为请求附加签名信息
type Signer interface {
	Sign(req *HttpRequest, config map[string]any) error
}

var signers = map[string]Signer{
	"basic":  &basicSigner{},
	"bearer": &bearerSigner{},
}
var signersMutex sync.RWMutex

var jsRunner func(ctx context.Context, code string, args map[string]any) (map[string]any, error)

// SetJSRunner 设置 JS 执行器钩子，由 go/js 引擎在启动时注入。
// 这避免了 api 模块直接依赖 go/js 产生的循环引用。
func SetJSRunner(runner func(context.Context, string, map[string]any) (map[string]any, error)) {
	jsRunner = runner
}

// RegisterSigner 注册全局签名器
func RegisterSigner(name string, s Signer) {
	signersMutex.Lock()
	signers[name] = s
	signersMutex.Unlock()
}

// RegisterJSSigner registers an administrator-managed JavaScript signer.
func RegisterJSSigner(name, code string) {
	RegisterSigner(name, &jsSigner{code: code})
}

// RemoveSigner unregisters a custom signer. Built-in signers are retained.
func RemoveSigner(name string) {
	if name == "basic" || name == "bearer" || name == "none" {
		return
	}
	signersMutex.Lock()
	delete(signers, name)
	signersMutex.Unlock()
}

// GetSigner 获取签名器
func GetSigner(name string) Signer {
	signersMutex.RLock()
	signer := signers[name]
	signersMutex.RUnlock()
	return signer
}

// jsSigner 包装 JS 代码为 Go Signer 接口
type jsSigner struct {
	code string
}

func (s *jsSigner) Sign(req *HttpRequest, config map[string]any) error {
	if jsRunner == nil {
		return errors.New("jsRunner is not set, cannot execute js signer")
	}

	// 准备传递给 JS 的参数
	args := map[string]any{
		"method":  req.Method,
		"url":     req.Url,
		"payload": req.Payload,
		"headers": req.headers,
		"config":  config,
	}

	// 执行 JS 签名逻辑
	// 注意：这里默认使用 Background context，除非底层能透传，低代码环境中会由 js_export 注入正确的 ctx
	ctx := req.Context
	if ctx == nil {
		ctx = context.Background()
	}
	res, err := jsRunner(ctx, s.code, args)
	if err != nil {
		return err
	}

	// 将结果（通常是修改后的 headers）写回请求对象
	if newHeaders, ok := res["headers"].(map[string]any); ok {
		for k, v := range newHeaders {
			req.SetHeader(k, v)
		}
	}

	// 如果 JS 返回了新的 URL 或 Method，也支持修改
	if newUrl, ok := res["url"].(string); ok {
		req.Url = newUrl
	}
	if newMethod, ok := res["method"].(string); ok {
		req.Method = newMethod
	}

	return nil
}

// 快速应用签名
func sign(name string, req *HttpRequest, config map[string]any) error {
	if name == "" || name == "none" {
		return nil
	}
	s := GetSigner(name)
	if s == nil {
		return errors.New("signer not found: " + name)
	}
	return s.Sign(req, config)
}

// 内置标准签名器实现

type basicSigner struct{}

func (s *basicSigner) Sign(req *HttpRequest, config map[string]any) error {
	req.SetHeader("Authorization", "Basic ", safe.Base64(config["username"], ":", config["password"]))
	return nil
}

type bearerSigner struct{}

func (s *bearerSigner) Sign(req *HttpRequest, config map[string]any) error {
	token := config["token"]
	if token == nil {
		token = config["key"]
	}

	req.SetHeader("Authorization", "Bearer ", token)
	return nil
}
