package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/url"
	"reflect"
	"runtime/debug"
	"strings"
	"time"

	"apigo.cc/go/cast"
	gohttp "apigo.cc/go/http"
	"apigo.cc/go/safe"
)

// Call invokes an Action and returns the common API result envelope.
func Call(action Action, options ...*CallOptions) (*Result, error) {
	if action == nil {
		return nil, errors.New("action is required")
	}
	opts := firstOptions(options)
	started := time.Now()
	result := &Result{Headers: map[string]string{}}
	finishTiming := func() {}
	if opts.Timing {
		result.Timing = map[string]any{"unit": "ms"}
		finishTiming = func() { result.Timing["total"] = time.Since(started).Milliseconds() }
		defer finishTiming()
	}
	actionConfig, _ := GetActionConfig(action.ActionName())
	if ca, ok := action.(ConfigurableAction); ok {
		MergeMap(actionConfig, ca.Config())
	}
	applyActionTraits(action, actionConfig)
	if enabled, exists := actionConfig["enabled"]; exists && !cast.Bool(enabled) {
		result.Error = "API Action is disabled"
		return result, errors.New(result.Error)
	}
	if cast.Bool(actionConfig["abstract"]) {
		result.Error = "abstract API Action cannot be called"
		return result, errors.New(result.Error)
	}
	if err := verifyToken(actionConfig, opts.Token); err != nil {
		result.Error = err.Error()
		logCall(opts, actionConfig, action.ActionName(), "", "", nil, result, started)
		return result, err
	}
	if err := mergeOverrides(actionConfig, opts.Config); err != nil {
		result.Error = err.Error()
		logCall(opts, actionConfig, action.ActionName(), "", "", nil, result, started)
		return result, err
	}

	var openedSecrets []*safe.SecretPlaintext
	preprocessCallSecrets(actionConfig, &openedSecrets)
	defer func() {
		for _, secret := range openedSecrets {
			secret.Close()
		}
	}()
	if va, ok := action.(ValidatableAction); ok {
		if err := va.Validate(); err != nil {
			result.Error = fmt.Sprintf("action validation failed: %v", err)
			return result, errors.New(result.Error)
		}
	}
	fill(action, actionConfig)
	method := strings.ToUpper(cast.String(actionConfig["method"]))
	if method == "" {
		method = "POST"
	}
	requestURL, err := buildURL(actionConfig)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	httpReq := &HttpRequest{Url: requestURL, Method: method, Payload: action, Context: opts.Context}
	defer httpReq.Close()
	if ga, ok := action.(*GenericAction); ok {
		httpReq.Payload = ga.payload
	}
	if headers, ok := actionConfig["headers"].(map[string]any); ok {
		for k, v := range headers {
			httpReq.SetHeader(k, v)
		}
	}
	filters := newFilterPipeline(actionConfig, "")
	if filtered, filterErr := filters.apply(opts.Context, "request", map[string]any{"action": action.ActionName(), "request": requestMap(httpReq, actionConfig)}); filterErr != nil {
		result.Error = filterErr.Error()
		return result, filterErr
	} else if request, ok := filtered["request"].(map[string]any); ok {
		if filteredConfig := applyRequestMap(httpReq, request); filteredConfig != nil {
			actionConfig = filteredConfig
		}
	}
	resolvedPayload, protectedSecrets, err := resolveProtectedInputs(opts.Context, action.ActionName(), actionConfig, httpReq.Payload)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	openedSecrets = append(openedSecrets, protectedSecrets...)
	httpReq.Payload = resolvedPayload
	if cast.Bool(actionConfig["llm"]) {
		sanitizeLLMPayload(httpReq.Payload)
	}
	if err := sign(cast.String(actionConfig["signer"]), httpReq, actionConfig); err != nil {
		result.Error = "sign failed: " + err.Error()
		return result, errors.New(result.Error)
	}

	payload := formattedPayload(httpReq.Payload, actionConfig)
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = cast.Duration(actionConfig["timeout"])
	}
	webSocketTransport := isWebSocketTransport(actionConfig)
	if webSocketTransport {
		err = callWebSocket(httpReq, payload, opts.Stream, filters, action.ActionName(), opts.Context, actionConfig, result, started, timeout)
	} else if opts.Stream != nil {
		client := gohttp.NewClient(timeout)
		defer client.Destroy()
		err = callStream(client, httpReq, payload, opts.Stream, filters, action.ActionName(), opts.Context, result, started)
	} else {
		client := gohttp.NewClient(timeout)
		defer client.Destroy()
		err = callBuffered(client, httpReq, payload, opts.Context, result)
		if err == nil {
			applyResponseRules(actionConfig, result)
			if filtered, filterErr := filters.apply(opts.Context, "result", map[string]any{"action": action.ActionName(), "result": resultMap(result)}); filterErr != nil {
				err = filterErr
				result.Error = filterErr.Error()
			} else if value, ok := filtered["result"].(map[string]any); ok {
				applyResultMap(result, value)
			}
		}
	}
	if err == nil && (opts.Stream != nil || webSocketTransport) {
		applyResponseRules(actionConfig, result)
		if filtered, filterErr := filters.apply(opts.Context, "done", map[string]any{"action": action.ActionName(), "result": resultMap(result)}); filterErr != nil {
			err = filterErr
			result.Error = filterErr.Error()
		} else if value, ok := filtered["result"].(map[string]any); ok {
			applyResultMap(result, value)
		}
	}
	if err == nil && opts.Stream != nil && opts.Stream.OnDone != nil {
		finishTiming()
		err = opts.Stream.OnDone(resultMap(result))
		if err != nil {
			result.Ok = false
			result.Error = err.Error()
		}
	}
	logCall(opts, actionConfig, action.ActionName(), httpReq.Method, httpReq.Url, httpReq.Payload, result, started)
	if err != nil {
		return result, err
	}
	return result, nil
}

func sanitizeLLMPayload(payload any) {
	values, ok := payload.(map[string]any)
	if !ok {
		return
	}
	for _, field := range []string{"name", "title", "extends", "abstract", "creation", "providerProtocol", "url", "host", "baseUrl", "path", "method", "format", "signer", "key", "filters", "requestSchema", "responseSchema", "test", "models", "defaultModel", "reasoningSupported"} {
		delete(values, field)
	}
}

func applyActionTraits(action Action, config map[string]any) {
	if value, ok := action.(URLAction); ok && value.GetURL() != "" {
		config["url"] = value.GetURL()
	}
	if value, ok := action.(MethodAction); ok && value.GetMethod() != "" {
		config["method"] = value.GetMethod()
	}
	if value, ok := action.(SignerAction); ok && value.SignerName() != "" {
		config["signer"] = value.SignerName()
	}
	if value, ok := action.(FormatAction); ok && value.GetFormat() != "" {
		config["format"] = value.GetFormat()
	}
}

// CallBy invokes a registered dynamic or Go Action.
func CallBy(name string, payload any, options ...*CallOptions) (*Result, error) {
	actionRegistryMutex.RLock()
	tmpl, ok := actionRegistry[name]
	actionRegistryMutex.RUnlock()
	if !ok {
		err := fmt.Errorf("action not found: %s", name)
		return &Result{Headers: map[string]string{}, Error: err.Error()}, err
	}
	var action Action
	if t, ok := tmpl.(reflect.Type); ok {
		inst := reflect.New(t).Interface()
		if payload != nil {
			cast.Convert(inst, payload)
		}
		action = inst.(Action)
	} else if ga, ok := tmpl.(*GenericAction); ok {
		config, resolveErr := resolveGenericActionConfig(name, map[string]bool{})
		if resolveErr != nil {
			return &Result{Headers: map[string]string{}, Error: resolveErr.Error()}, resolveErr
		}
		copyAction := &GenericAction{name: ga.name, config: config, payload: cloneMap(ga.payload)}
		if payload != nil {
			if m, ok := payload.(map[string]any); ok {
				MergeMap(copyAction.payload, m)
			} else {
				cast.Convert(&copyAction.payload, payload)
			}
		}
		action = copyAction
	}
	return Call(action, options...)
}

func resolveGenericActionConfig(name string, visiting map[string]bool) (map[string]any, error) {
	if visiting[name] {
		return nil, fmt.Errorf("circular Action inheritance: %s", name)
	}
	actionRegistryMutex.RLock()
	template, exists := actionRegistry[name]
	actionRegistryMutex.RUnlock()
	generic, ok := template.(*GenericAction)
	if !exists || !ok {
		return nil, fmt.Errorf("inherited Action not found: %s", name)
	}
	visiting[name] = true
	defer delete(visiting, name)
	config := map[string]any{}
	if parent := cast.String(generic.config["extends"]); parent != "" {
		parentConfig, err := resolveGenericActionConfig(parent, visiting)
		if err != nil {
			return nil, err
		}
		// abstract describes whether this definition itself is callable. Children
		// inherit its configuration contract, but become concrete by definition.
		delete(parentConfig, "abstract")
		MergeMap(config, parentConfig)
	}
	MergeMap(config, generic.config)
	return config, nil
}

func formattedPayload(payload any, config map[string]any) any {
	switch strings.ToLower(cast.String(config["format"])) {
	case "form":
		var form gohttp.Form
		cast.Convert(&form, payload)
		return form
	case "multipart":
		var multipart gohttp.Multipart
		cast.Convert(&multipart, payload)
		return multipart
	default:
		return payload
	}
}

func callBuffered(client *gohttp.Client, req *HttpRequest, payload any, ctx context.Context, result *Result) error {
	res := client.DoContext(ctx, req.Method, req.Url, payload, headerSlice(req)...)
	if res.Error != nil {
		result.Error = res.Error.Error()
		return res.Error
	}
	copyHTTPMeta(res.Response, result)
	if res.Response != nil && res.Response.ContentLength > 0 {
		result.receivedBytes = res.Response.ContentLength
	}
	var data any
	if err := res.To(&data); err != nil {
		result.Error = err.Error()
		return err
	}
	result.Data = data
	// Keep non-success provider bodies available to higher-level filters. They
	// may normalize the public result, but diagnostics still need the raw body.
	if result.StatusCode >= 400 {
		if object, ok := data.(map[string]any); ok {
			raw := map[string]any{}
			for key, value := range object {
				raw[key] = value
			}
			result.ErrorDetail = raw
			object["_providerError"] = raw
		} else {
			result.ErrorDetail = data
		}
	}
	return nil
}

func callStream(client *gohttp.Client, req *HttpRequest, payload any, stream *StreamOptions, filters *filterPipeline, action string, ctx context.Context, result *Result, started time.Time) error {
	res := client.ManualDoContext(ctx, req.Method, req.Url, payload, headerSlice(req)...)
	if res.Error != nil {
		result.Error = res.Error.Error()
		return res.Error
	}
	if res.Response == nil {
		result.Error = "empty HTTP response"
		return errors.New(result.Error)
	}
	defer res.Response.Body.Close()
	copyHTTPMeta(res.Response, result)
	if result.StatusCode >= 400 {
		body, readErr := io.ReadAll(res.Response.Body)
		if len(body) > 0 {
			var detail any
			if decodeErr := cast.UnmarshalJSON(body, &detail); decodeErr == nil {
				result.ErrorDetail = detail
				result.Data = detail
			}
		}
		if result.Error == "" {
			result.Error = fmt.Sprintf("HTTP status %d", result.StatusCode)
		}
		if readErr != nil {
			result.Error = readErr.Error()
		}
		return errors.New(result.Error)
	}
	if filtered, err := filters.apply(ctx, "headers", map[string]any{"action": action, "result": resultMap(result)}); err != nil {
		result.Error = err.Error()
		return err
	} else if value, ok := filtered["result"].(map[string]any); ok {
		applyResultMap(result, value)
	}
	if stream.OnHeaders != nil {
		if err := invokeStreamCallback(func() error { return stream.OnHeaders(resultMap(result)) }); err != nil {
			result.Error = err.Error()
			return err
		}
	}
	buf := make([]byte, 32*1024)
	for {
		n, readErr := res.Response.Body.Read(buf)
		if n > 0 {
			if result.Timing != nil {
				if _, exists := result.Timing["firstToken"]; !exists {
					result.Timing["firstToken"] = time.Since(started).Milliseconds()
				}
			}
			result.receivedBytes += int64(n)
			if stream.OnData != nil {
				filtered, filterErr := filters.apply(ctx, "chunk", map[string]any{"action": action, "chunk": append([]byte(nil), buf[:n]...), "drop": false})
				if filterErr != nil {
					result.Error = filterErr.Error()
					return filterErr
				}
				chunk := filteredChunk(filtered["chunk"])
				if !cast.Bool(filtered["drop"]) && len(chunk) > 0 {
					if err := invokeStreamCallback(func() error { return stream.OnData(chunk) }); err != nil {
						result.Error = err.Error()
						return err
					}
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			result.Error = readErr.Error()
			return readErr
		}
	}
	return nil
}

func resultMap(result *Result) map[string]any {
	if result == nil {
		return map[string]any{"ok": false, "statusCode": 0, "headers": map[string]string{}, "data": nil, "code": "", "error": "empty API result"}
	}
	output := map[string]any{
		"ok": result.Ok, "statusCode": result.StatusCode, "headers": result.Headers,
		"data": result.Data, "code": result.Code, "error": result.Error, "errorDetail": result.ErrorDetail,
	}
	if result.Timing != nil {
		output["timing"] = result.Timing
	}
	return output
}

func invokeStreamCallback(callback func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("stream callback panic: %v\n%s", recovered, debug.Stack())
		}
	}()
	return callback()
}

func copyHTTPMeta(response *stdhttp.Response, result *Result) {
	if response == nil {
		return
	}
	result.StatusCode = response.StatusCode
	for key, values := range response.Header {
		result.Headers[key] = strings.Join(values, ", ")
	}
}

func buildURL(config map[string]any) (string, error) {
	raw := cast.String(config["url"])
	path := cast.String(config["path"])
	if raw == "" {
		raw = cast.String(config["baseUrl"])
		if raw == "" {
			raw = cast.String(config["host"])
			if raw != "" && !strings.Contains(raw, "://") {
				raw = "https://" + raw
			}
		}
		if path != "" {
			raw = strings.TrimRight(raw, "/") + "/" + strings.TrimLeft(path, "/")
		}
	}
	if raw == "" {
		return "", errors.New("API URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if cast.String(config["url"]) != "" && path != "" {
		u.Path = "/" + strings.TrimLeft(path, "/")
		u.RawPath = ""
		raw = u.String()
	}
	if values, ok := config["query"].(map[string]any); ok && len(values) > 0 {
		query := u.Query()
		for key, value := range values {
			query.Set(key, secretString(value))
		}
		u.RawQuery = query.Encode()
		return u.String(), nil
	}
	return raw, nil
}

func preprocessCallSecrets(m map[string]any, opened *[]*safe.SecretPlaintext) {
	for k, v := range m {
		if sb, ok := v.(*safe.SafeBuf); ok {
			secret := sb.Open()
			m[k] = secret
			*opened = append(*opened, secret)
		} else if subMap, ok := v.(map[string]any); ok {
			preprocessCallSecrets(subMap, opened)
		}
	}
}

func secretString(value any) string {
	if secret, ok := value.(*safe.SecretPlaintext); ok && secret != nil {
		return secret.String()
	}
	return cast.String(value)
}
