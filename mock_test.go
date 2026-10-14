package api_test

import (
	"apigo.cc/go/api"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestMockNeverContactsProviderAndRequiresDeclaredInputs(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sent":true}`))
	}))
	defer server.Close()
	name := "test.mock.sms"
	definition := map[string]any{"url": server.URL, "enabled": true, "tokens": []string{"allowed"}, "mock": map[string]any{"enabled": true, "request": map[string]any{"to": "15555550100"}, "response": map[string]any{"statusCode": 200, "data": map[string]any{"sent": true}}}}
	api.RegisterAction(name, definition)
	defer api.RemoveAction(name)
	payload := map[string]any{"to": "15555550100", "text": "test"}
	result, err := api.CallBy(name, payload, &api.CallOptions{Mode: "mock", Token: "allowed"})
	if err != nil || !result.Ok || !result.Simulated || result.Mode != "mock" || calls.Load() != 0 {
		t.Fatalf("mock failed: %+v %v calls=%d", result, err, calls.Load())
	}
	for _, opts := range []*api.CallOptions{{Mode: "mock"}, {Mode: "invalid", Token: "allowed"}, {Mode: "mock", Token: "allowed", Config: map[string]any{"mock": map[string]any{"enabled": true}}}, {Mode: "mock", Token: "allowed", Stream: &api.StreamOptions{}}} {
		if _, err = api.CallBy(name, payload, opts); err == nil {
			t.Fatalf("invalid options accepted: %+v", opts)
		}
	}
	if result, err = api.CallBy(name, map[string]any{"to": "15555550101"}, &api.CallOptions{Mode: "mock", Token: "allowed"}); err == nil {
		t.Fatal("undeclared test recipient accepted")
	}
	var rejected *api.MockInputError
	if !errors.As(err, &rejected) || result.Ok || result.Code != "mock_input_rejected" || calls.Load() != 0 {
		t.Fatalf("negative test outcome lost: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = api.CallBy(name, payload, &api.CallOptions{Mode: "mock", Token: "allowed", Context: ctx}); err == nil {
		t.Fatal("canceled mock accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("mock failure made network call")
	}
	result, err = api.CallBy(name, payload, &api.CallOptions{Token: "allowed"})
	if err != nil || !result.Ok || result.Simulated || calls.Load() != 1 {
		t.Fatalf("default live mode changed: %+v %v", result, err)
	}
	definition["enabled"] = false
	api.RegisterAction(name, definition)
	if _, err = api.CallBy(name, payload, &api.CallOptions{Mode: "mock", Token: "allowed"}); err == nil {
		t.Fatal("disabled action mocked")
	}
}

func TestMockPreservesProviderFailureAndInheritance(t *testing.T) {
	parent, child := "test.mock.parent", "test.mock.child"
	api.RegisterAction(parent, map[string]any{"abstract": true, "url": "https://example.invalid", "mock": map[string]any{"enabled": true, "request": map[string]any{"to": "test"}, "response": map[string]any{"statusCode": 429, "data": map[string]any{"reason": "quota"}}}})
	api.RegisterAction(child, map[string]any{"extends": parent, "abstract": false})
	defer api.RemoveAction(parent)
	defer api.RemoveAction(child)
	result, err := api.CallBy(child, map[string]any{"to": "test"}, &api.CallOptions{Mode: "mock"})
	if err != nil || result.Ok || !result.Simulated || result.StatusCode != 429 || result.Error == "" {
		t.Fatalf("failure fixture lost: %+v %v", result, err)
	}
}
