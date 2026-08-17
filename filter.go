package api

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"apigo.cc/go/cast"
)

// Filter transforms one request or response event. Implementations must be stateless;
// per-call state is passed through input["state"] and returned as output["state"].
type Filter interface {
	Apply(ctx context.Context, event string, input map[string]any) (map[string]any, error)
}

var filterRegistry = map[string]Filter{}
var filterRegistryMutex sync.RWMutex

// RegisterFilter registers a reusable request/response filter.
func RegisterFilter(name string, filter Filter) {
	if name == "" || filter == nil {
		return
	}
	filterRegistryMutex.Lock()
	filterRegistry[name] = filter
	filterRegistryMutex.Unlock()
}

// RegisterJSFilter registers an administrator-managed JavaScript filter.
func RegisterJSFilter(name, code string) { RegisterFilter(name, &jsFilter{code: code}) }

// RemoveFilter unregisters a custom filter.
func RemoveFilter(name string) {
	filterRegistryMutex.Lock()
	delete(filterRegistry, name)
	filterRegistryMutex.Unlock()
}

// GetFilter returns one registered filter.
func GetFilter(name string) Filter {
	filterRegistryMutex.RLock()
	filter := filterRegistry[name]
	filterRegistryMutex.RUnlock()
	return filter
}

type jsFilter struct{ code string }

func (filter *jsFilter) Apply(ctx context.Context, event string, input map[string]any) (map[string]any, error) {
	if jsRunner == nil {
		return nil, errors.New("jsRunner is not set, cannot execute JS filter")
	}
	args := cloneMap(input)
	args["event"] = event
	if event == "chunk" {
		if chunk, ok := args["chunk"].([]byte); ok {
			args["chunk"] = string(chunk)
		}
	}
	return jsRunner(ctx, filter.code, args)
}

type filterPipeline struct {
	names  []string
	states map[string]map[string]any
}

func newFilterPipeline(config map[string]any, _ string) *filterPipeline {
	names := stringList(config["filters"])
	if len(names) == 0 {
		return nil
	}
	return &filterPipeline{names: names, states: map[string]map[string]any{}}
}

func (pipeline *filterPipeline) apply(ctx context.Context, event string, input map[string]any) (map[string]any, error) {
	if pipeline == nil {
		return input, nil
	}
	current := cloneMap(input)
	for _, name := range pipeline.names {
		filter := GetFilter(name)
		if filter == nil {
			return nil, fmt.Errorf("filter not found: %s", name)
		}
		current["filter"] = name
		current["state"] = pipeline.states[name]
		output, err := filter.Apply(ctx, event, current)
		if err != nil {
			return nil, fmt.Errorf("filter %s %s failed: %w", name, event, err)
		}
		if output == nil {
			continue
		}
		if state, ok := output["state"].(map[string]any); ok {
			pipeline.states[name] = state
		}
		for key, value := range output {
			if key != "state" {
				current[key] = value
			}
		}
		if cast.Bool(current["drop"]) {
			break
		}
	}
	delete(current, "filter")
	delete(current, "state")
	return current, nil
}

func requestMap(request *HttpRequest, config map[string]any) map[string]any {
	headers := map[string]any{}
	for key, value := range request.headers {
		headers[key] = value
	}
	return map[string]any{
		"url": request.Url, "method": request.Method, "headers": headers,
		"payload": request.Payload, "config": config,
	}
}

func applyRequestMap(request *HttpRequest, value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	request.Url = cast.String(value["url"])
	request.Method = cast.String(value["method"])
	if payload, exists := value["payload"]; exists {
		request.Payload = payload
	}
	if headers, ok := value["headers"].(map[string]any); ok {
		request.headers = map[string]string{}
		for key, header := range headers {
			request.SetHeader(key, header)
		}
	}
	config, _ := value["config"].(map[string]any)
	return config
}

func applyResultMap(result *Result, value map[string]any) {
	if result == nil || value == nil {
		return
	}
	if item, exists := value["ok"]; exists {
		result.Ok = cast.Bool(item)
	}
	if item, exists := value["statusCode"]; exists {
		result.StatusCode = cast.Int(item)
	}
	if item, exists := value["data"]; exists {
		result.Data = item
	}
	if item, exists := value["code"]; exists {
		result.Code = cast.String(item)
	}
	if item, exists := value["error"]; exists {
		result.Error = cast.String(item)
	}
	if headers, ok := value["headers"].(map[string]any); ok {
		result.Headers = map[string]string{}
		for key, header := range headers {
			result.Headers[key] = cast.String(header)
		}
	}
}

func filteredChunk(value any) []byte {
	switch chunk := value.(type) {
	case []byte:
		return chunk
	case string:
		return []byte(chunk)
	case nil:
		return nil
	default:
		return []byte(cast.String(chunk))
	}
}
