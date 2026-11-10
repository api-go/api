package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/api-go/jsmod"
	"github.com/api-go/safe"
)

type testProtectedResolver struct{ value *safe.SafeBuf }

type protectedInputTestFilter func(context.Context, string, map[string]any) (map[string]any, error)

func (filter protectedInputTestFilter) Apply(ctx context.Context, event string, input map[string]any) (map[string]any, error) {
	return filter(ctx, event, input)
}

func (resolver *testProtectedResolver) OpenProtected(reference string) (*safe.SecretPlaintext, error) {
	if reference != "@protected:test-phone" {
		return nil, errors.New("invalid protected reference")
	}
	return resolver.value.Open(), nil
}

func TestJSCallResolvesDeclaredProtectedInput(t *testing.T) {
	received := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		received <- payload
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	action := t.Name() + ".sms.send"
	RegisterAction(action, map[string]any{
		"url": server.URL, "method": "POST", "format": "json",
		"protectedInputs": []any{"phone"},
	})
	defer RemoveAction(action)
	resolver := &testProtectedResolver{value: safe.NewSafeBuf([]byte("13800138000"))}
	defer resolver.value.Close()
	ctx := jsmod.NewContext(context.Background(), map[string]any{ProtectedValuesContextKey: resolver})

	if _, err := call(ctx, action, map[string]any{"phone": "@protected:test-phone", "message": "test"}); err != nil {
		t.Fatal(err)
	}
	if got := (<-received)["phone"]; got != "13800138000" {
		t.Fatalf("phone = %#v", got)
	}
}

func TestJSCallDoesNotResolveUndeclaredProtectedInput(t *testing.T) {
	resolved, opened, err := resolveProtectedInputs(context.Background(), "missing", nil, map[string]any{"phone": "@protected:test-phone"})
	if err != nil || len(opened) != 0 {
		t.Fatalf("resolveProtectedInputs error = %v, opened = %d", err, len(opened))
	}
	if got := resolved.(map[string]any)["phone"]; got != "@protected:test-phone" {
		t.Fatalf("phone = %#v", got)
	}
}

func TestProtectedInputRemainsOpaqueToFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	seen := ""
	filterName := t.Name() + ".filter"
	RegisterFilter(filterName, protectedInputTestFilter(func(_ context.Context, event string, input map[string]any) (map[string]any, error) {
		if event == "request" {
			request := input["request"].(map[string]any)
			seen = request["payload"].(map[string]any)["phone"].(string)
		}
		return input, nil
	}))
	action := t.Name() + ".sms.send"
	RegisterAction(action, map[string]any{
		"url": server.URL, "method": "POST", "format": "json",
		"filters": []any{filterName}, "protectedInputs": []any{"phone"},
	})
	defer RemoveAction(action)
	resolver := &testProtectedResolver{value: safe.NewSafeBuf([]byte("13800138000"))}
	defer resolver.value.Close()
	ctx := jsmod.NewContext(context.Background(), map[string]any{ProtectedValuesContextKey: resolver})
	if _, err := call(ctx, action, map[string]any{"phone": "@protected:test-phone"}); err != nil {
		t.Fatal(err)
	}
	if seen != "@protected:test-phone" {
		t.Fatalf("filter saw %q", seen)
	}
}
