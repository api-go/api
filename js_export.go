package api

import (
	"context"

	"apigo.cc/go/jsmod"
)

func init() {
	jsmod.Register("api", map[string]any{
		"Call":           call,
		"SetConfig":      SetConfig,
		"RegisterAction": RegisterAction,
		"RegisterSigner": registerSigner,
	})
}

// call 提供给 JS 的私有入口
func call(ctx context.Context, name string, payload any) (any, error) {
	// 将 ctx 传入以透传追踪信息给 JS 签名器
	return CallBy[any](name, payload)
}

// registerSigner 允许从 JS 注册动态签名逻辑
func registerSigner(name string, code string) {
	RegisterSigner(name, &jsSigner{code: code})
}
