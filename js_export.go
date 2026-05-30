package api

import (
	"context"

	"apigo.cc/go/jsmod"
)

func init() {
	jsmod.Register("api", map[string]any{
		"call":           call,
		"setConfig":      SetConfig,
		"registerAction": RegisterAction,
		"registerSigner": registerSigner,
	})
}

// call 提供给 JS 的私有入口
func call(ctx context.Context, name string, payload any) (any, error) {
	// 注意：在低代码环境中，我们可能需要将 ctx 传入以透传追踪信息给 JS 签名器。
	// 虽然 api.CallBy[any] 目前不接收 ctx，但底层的 jsRunner 钩子可以从 context 中获取信息。
	// 如果未来 jsRunner 需要 ctx，我们可以在这里通过 context.WithValue 注入。
	return CallBy[any](name, payload)
}

// registerSigner 允许从 JS 注册动态签名逻辑
func registerSigner(name string, code string) {
	RegisterSigner(name, &jsSigner{code: code})
}
