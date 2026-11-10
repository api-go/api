package api

import (
	"context"

	"github.com/api-go/cast"
	"github.com/api-go/jsmod"
)

func init() {
	jsmod.Register("api", map[string]any{
		"Call":           call,
		"SetConfig":      SetConfig,
		"RegisterAction": RegisterAction,
		"RegisterSigner": registerSigner,
		"Encrypt":        Encrypt,
	}, "SetConfig", "RegisterAction", "RegisterSigner")
}

// call 提供给 JS 的私有入口
func call(ctx context.Context, name string, payload any, options ...*CallOptions) (any, error) {
	// 将 ctx 传入以透传追踪信息给 JS 签名器
	opts := firstOptions(options)
	opts.Context = ctx
	res, err := CallBy(name, payload, opts)
	if actionUsesDataResult(name) {
		return projectDataResult(res), nil
	}
	if err != nil {
		if res != nil {
			return resultMap(res), nil
		}
		return nil, jsmod.MakeError(err)
	}
	return resultMap(res), nil
}

func actionUsesDataResult(name string) bool {
	config, err := resolveGenericActionConfig(name, map[string]bool{})
	return err == nil && cast.String(config["resultMode"]) == "data"
}

func projectDataResult(result *Result) any {
	if result == nil {
		return map[string]any{"ok": false, "code": "", "error": "empty API result"}
	}
	if data, ok := result.Data.(map[string]any); ok {
		output := cloneMap(data)
		if result.Simulated {
			output["mode"], output["simulated"] = result.Mode, true
		}
		if _, exists := output["ok"]; !exists {
			output["ok"] = result.Ok
		}
		if _, exists := output["code"]; !exists {
			output["code"] = result.Code
		}
		if _, exists := output["error"]; !exists {
			output["error"] = result.Error
		}
		if result.Timing != nil {
			output["timing"] = result.Timing
		}
		return output
	}
	output := map[string]any{
		"ok": result.Ok, "code": result.Code, "error": result.Error, "result": result.Data,
	}
	if result.Simulated {
		output["mode"], output["simulated"] = result.Mode, true
	}
	if result.Timing != nil {
		output["timing"] = result.Timing
	}
	return output
}

// registerSigner 允许从 JS 注册动态签名逻辑
func registerSigner(name string, code string) {
	RegisterSigner(name, &jsSigner{code: code})
}
