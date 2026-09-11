package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"apigo.cc/go/cast"
	"apigo.cc/go/jsmod"
	"apigo.cc/go/safe"
)

// ProtectedValuesContextKey identifies the request-scoped protected-value resolver.
const ProtectedValuesContextKey = "ProtectedValues"

// ProtectedValueResolver opens references owned by the current low-code call.
type ProtectedValueResolver interface {
	OpenProtected(reference string) (*safe.SecretPlaintext, error)
}

type protectedString string

func resolveProtectedInputs(ctx context.Context, action string, config map[string]any, payload any) (any, []*safe.SecretPlaintext, error) {
	paths := stringList(config["protectedInputs"])
	if len(paths) == 0 {
		return payload, nil, nil
	}
	values, ok := payload.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("API Action %s protectedInputs require an object payload", action)
	}
	resolved := cloneMap(values)
	opened := make([]*safe.SecretPlaintext, 0, len(paths))
	for _, path := range paths {
		container, key, exists := protectedInputLocation(resolved, path)
		if !exists {
			continue
		}
		reference := cast.String(container[key])
		if !strings.HasPrefix(reference, "@protected:") {
			continue
		}
		resolver, ok := jsmod.Get(ctx, ProtectedValuesContextKey).(ProtectedValueResolver)
		if !ok || resolver == nil {
			closeProtectedInputs(opened)
			return nil, nil, errors.New("protected values are unavailable in this call context")
		}
		plaintext, openErr := resolver.OpenProtected(reference)
		if openErr != nil {
			closeProtectedInputs(opened)
			return nil, nil, fmt.Errorf("protected input %s: %w", path, openErr)
		}
		opened = append(opened, plaintext)
		container[key] = protectedString(plaintext.String())
	}
	return resolved, opened, nil
}

func protectedInputLocation(payload map[string]any, path string) (map[string]any, string, bool) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, "", false
	}
	current := payload
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			return nil, "", false
		}
		current = next
	}
	key := parts[len(parts)-1]
	_, exists := current[key]
	return current, key, exists
}

func closeProtectedInputs(values []*safe.SecretPlaintext) {
	for _, value := range values {
		value.Close()
	}
}
