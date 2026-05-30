package api

import (
	"context"

	"apigo.cc/go/cast"
	"apigo.cc/go/jsmod"
)

func init() {
	jsmod.Register("api", map[string]any{
		"call":      call,
		"addConfig": AddConfig,
	})
}

// call 提供给 JS 的私有入口，避免污染 Go 公开 API
func call(ctx context.Context, actionName string, payload any, options ...map[string]any) (any, error) {
	var opt map[string]any
	if len(options) > 0 {
		opt = options[0]
	}

	if payload == nil {
		payload = make(map[string]any)
	}

	// 1. 预填 Payload (JS 传来的 map 需要手动触发一次 fill 以获取全局配置)
	actionConfig, safeBufs := GetActionConfig(actionName)
	defer func() {
		for _, sb := range safeBufs {
			sb.Close()
		}
	}()
	fill(payload, actionConfig)

	// 2. 构造符合 Action 接口的私有包装器
	ja := &jsAction{
		name:    actionName,
		payload: payload,
		options: opt,
	}

	return Call[any](ja)
}

type jsAction struct {
	name    string
	payload any
	options map[string]any
}

func (a *jsAction) ActionName() string { return a.name }
func (a *jsAction) SignerName() string {
	if a.options != nil {
		return cast.String(a.options["signer"])
	}
	return ""
}
func (a *jsAction) GetURL() string    { return cast.String(a.options["url"]) }
func (a *jsAction) GetMethod() string { return cast.String(a.options["method"]) }
func (a *jsAction) Config() map[string]any {
	return a.options
}
func (a *jsAction) MarshalJSON() ([]byte, error) {
	return cast.ToJSONBytes(a.payload)
}
