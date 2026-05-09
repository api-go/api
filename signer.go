package api

import (
	"errors"

	"apigo.cc/go/safe"
)

// Signer 负责为请求附加签名信息
type Signer interface {
	Sign(req *HttpRequest, config map[string]any) error
}

var signers = map[string]Signer{
	"basic":  &basicSigner{},
	"bearer": &bearerSigner{},
}

// RegisterSigner 注册全局签名器
func RegisterSigner(name string, s Signer) {
	signers[name] = s
}

// GetSigner 获取签名器
func GetSigner(name string) Signer {
	return signers[name]
}

// 快速应用签名
func sign(name string, req *HttpRequest, config map[string]any) error {
	if name == "" {
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
