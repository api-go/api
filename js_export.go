package api

import (
	"context"

	"apigo.cc/go/cast"
	"apigo.cc/go/jsmod"
)

func init() {
	jsmod.Register("api", map[string]any{
		"call": CallAny,
		"load": Load,
	})
}

func CallAny(ctx context.Context, actionName string, payload any, options ...map[string]any) (any, error) {
	var opt map[string]any
	if len(options) > 0 {
		opt = options[0]
	}

	if payload == nil {
		payload = make(map[string]any)
	}

	// 1. Pre-fill the payload from GlobalConfigs (manually doing what api.Call does later)
	actionConfig, safeBufs := GetActionConfig(actionName)
	defer func() {
		for _, sb := range safeBufs {
			sb.Close()
		}
	}()
	fill(payload, actionConfig)

	// 2. Wrap into jsAction for interface satisfaction
	ja := &jsAction{
		name:    actionName,
		payload: payload,
		options: opt,
	}

	return Call[any](ja)
}

// jsAction implements Action, SignerAction, URLAction, MethodAction, and ConfigurableAction.
// It redirects reflection-based 'fill' and 'json.Marshal' to its internal payload.
type jsAction struct {
	name    string
	payload any
	options map[string]any
}

func (a *jsAction) ActionName() string { return a.name }

func (a *jsAction) SignerName() string {
	if a.options != nil {
		if s := cast.String(a.options["signer"]); s != "" {
			return s
		}
	}
	return ""
}

func (a *jsAction) GetURL() string {
	if a.options != nil {
		if u := cast.String(a.options["url"]); u != "" {
			return u
		}
	}
	return ""
}

func (a *jsAction) GetMethod() string {
	if a.options != nil {
		if m := cast.String(a.options["method"]); m != "" {
			return m
		}
	}
	return ""
}

func (a *jsAction) Config() map[string]any {
	return a.options
}

// MarshalJSON ensures that when this action is sent via HTTP (as JSON),
// only the internal payload is sent, not the metadata fields.
func (a *jsAction) MarshalJSON() ([]byte, error) {
	return cast.ToJSONBytes(a.payload)
}
