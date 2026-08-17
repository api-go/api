package api

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"apigo.cc/go/cast"
	"apigo.cc/go/safe"
)

var overrideBlacklist = map[string]bool{
	"tokens": true, "enabled": true, "test": true, "logging": true,
}

func verifyToken(config map[string]any, token string) error {
	raw, exists := config["tokens"]
	if !exists || raw == nil {
		return nil
	}
	tokens := make([]string, 0)
	switch values := raw.(type) {
	case []string:
		for _, value := range values {
			if value != "" {
				tokens = append(tokens, value)
			}
		}
	case []any:
		for _, value := range values {
			if token := cast.String(value); token != "" {
				tokens = append(tokens, token)
			}
		}
	default:
		v := reflect.ValueOf(raw)
		if v.IsValid() && (v.Kind() == reflect.Array || v.Kind() == reflect.Slice) {
			for i := 0; i < v.Len(); i++ {
				tokens = append(tokens, cast.String(v.Index(i).Interface()))
			}
		}
	}
	if len(tokens) == 0 {
		return nil
	}
	for _, allowed := range tokens {
		if allowed == token {
			return nil
		}
	}
	return errors.New("API token is required or invalid")
}

func mergeOverrides(config, overrides map[string]any) error {
	if len(overrides) == 0 {
		return nil
	}
	return mergeAllowed(config, overrides, "")
}

func mergeAllowed(dst, src map[string]any, prefix string) error {
	for key, value := range src {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if overrideBlacklist[key] {
			return fmt.Errorf("configuration field %s cannot be overridden", path)
		}
		current, exists := dst[key]
		if exists && isSecretValue(current) {
			return fmt.Errorf("encrypted configuration field %s cannot be overridden", path)
		}
		if child, ok := value.(map[string]any); ok {
			currentMap, _ := current.(map[string]any)
			if currentMap == nil {
				currentMap = map[string]any{}
				dst[key] = currentMap
			}
			if err := mergeAllowed(currentMap, child, path); err != nil {
				return err
			}
			continue
		}
		dst[key] = value
	}
	return nil
}

func isSecretValue(value any) bool {
	switch t := value.(type) {
	case string:
		return strings.HasPrefix(t, "**")
	case *safe.SafeBuf, *safe.SecretPlaintext:
		return true
	default:
		return false
	}
}

func applyResponseRules(config map[string]any, result *Result) {
	response, _ := config["response"].(map[string]any)
	result.Code = firstPathString(result.Data, response["codeFields"])
	if result.Error == "" {
		result.Error = firstPathString(result.Data, response["errorFields"])
	}
	result.Ok = result.StatusCode >= 200 && result.StatusCode < 300 && result.Error == ""
	if (result.StatusCode < 200 || result.StatusCode >= 300) && result.Error == "" {
		result.Error = fmt.Sprintf("HTTP status %d", result.StatusCode)
	}
	if codes := stringList(response["successCodes"]); len(codes) > 0 {
		if result.Code == "" {
			result.Ok = false
			if result.Error == "" {
				result.Error = "success code field was not found"
			}
			return
		}
		result.Ok = false
		for _, code := range codes {
			if result.Code == code {
				result.Ok = result.Error == "" && result.StatusCode >= 200 && result.StatusCode < 300
				break
			}
		}
		if !result.Ok && result.Error == "" {
			result.Error = "unexpected response code: " + result.Code
		}
	}
}

func stringList(value any) []string {
	var result []string
	switch values := value.(type) {
	case []string:
		return append(result, values...)
	case []any:
		for _, item := range values {
			result = append(result, cast.String(item))
		}
	case string:
		if values != "" {
			result = append(result, values)
		}
	}
	return result
}

func firstPathString(data, fields any) string {
	for _, path := range stringList(fields) {
		if value, ok := pathValue(data, path); ok {
			if text := cast.String(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func pathValue(data any, path string) (any, bool) {
	current := data
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
