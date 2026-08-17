package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"apigo.cc/go/api"
)

type testFilter func(context.Context, string, map[string]any) (map[string]any, error)

func (filter testFilter) Apply(ctx context.Context, event string, input map[string]any) (map[string]any, error) {
	return filter(ctx, event, input)
}

func registerTestAction(t *testing.T, config map[string]any) string {
	t.Helper()
	name := "engine." + t.Name()
	api.GlobalConfigs = map[string]any{}
	api.RegisterAction(name, config)
	return name
}

func TestTokenAndOverridePolicy(t *testing.T) {
	name := registerTestAction(t, map[string]any{
		"url": "http://example.invalid", "tokens": []any{"one", "two"}, "secret": "**cipher",
	})
	if result, err := api.CallBy(name, nil); err == nil || result.Error == "" {
		t.Fatal("missing token should be rejected")
	}
	if result, err := api.CallBy(name, nil, &api.CallOptions{Token: "one", Config: map[string]any{"tokens": []any{}}}); err == nil || result.Error == "" {
		t.Fatal("tokens override should be rejected")
	}
	if result, err := api.CallBy(name, nil, &api.CallOptions{Token: "one", Config: map[string]any{"secret": "plain"}}); err == nil || result.Error == "" {
		t.Fatal("encrypted value override should be rejected")
	}
}

func TestResultRulesAndConfigOverride(t *testing.T) {
	var receivedPath, receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath, receivedMethod = r.URL.Path, r.Method
		w.Header().Set("X-Test", "yes")
		_, _ = w.Write([]byte(`{"code":"OK","payload":{"value":1}}`))
	}))
	defer server.Close()
	name := registerTestAction(t, map[string]any{
		"url": "http://example.invalid", "method": "POST",
		"response": map[string]any{"codeFields": []any{"code"}, "successCodes": []any{"OK"}},
	})
	result, err := api.CallBy(name, map[string]any{"hello": "world"}, &api.CallOptions{Config: map[string]any{"url": server.URL + "/old", "path": "/v1/new", "method": "PUT"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ok || result.Code != "OK" || result.StatusCode != 200 || result.Headers["X-Test"] != "yes" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if receivedPath != "/v1/new" || receivedMethod != "PUT" {
		t.Fatalf("override reached %s %s", receivedMethod, receivedPath)
	}
}

func TestStreamPreservesChunkOrder(t *testing.T) {
	chunks := [][]byte{[]byte("data: first\n\n"), []byte("data: second\n\n")}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		for _, chunk := range chunks {
			_, _ = w.Write(chunk)
			flusher.Flush()
		}
	}))
	defer server.Close()
	name := registerTestAction(t, map[string]any{"url": server.URL, "method": "GET"})
	var got bytes.Buffer
	var done bool
	result, err := api.CallBy(name, nil, &api.CallOptions{Stream: &api.StreamOptions{
		OnData: func(data []byte) error { _, _ = got.Write(data); return nil },
		OnDone: func(result map[string]any) error { done, _ = result["ok"].(bool); return nil },
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ok || !done || !reflect.DeepEqual(got.Bytes(), bytes.Join(chunks, nil)) {
		t.Fatalf("stream mismatch: ok=%v done=%v body=%q", result.Ok, done, got.String())
	}
}

func TestUnifiedFilterLifecycle(t *testing.T) {
	var receivedMethod, receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod, receivedHeader = r.Method, r.Header.Get("X-Filtered")
		_, _ = w.Write([]byte(`{"message":"raw"}`))
	}))
	defer server.Close()
	beforeName, afterName := t.Name()+".before", t.Name()+".after"
	api.RegisterFilter(beforeName, testFilter(func(_ context.Context, event string, input map[string]any) (map[string]any, error) {
		if event != "request" {
			return nil, nil
		}
		request := input["request"].(map[string]any)
		request["url"] = server.URL
		request["method"] = "PUT"
		request["headers"] = map[string]any{"X-Filtered": "yes"}
		return map[string]any{"request": request}, nil
	}))
	api.RegisterFilter(afterName, testFilter(func(_ context.Context, event string, input map[string]any) (map[string]any, error) {
		if event != "result" {
			return nil, nil
		}
		result := input["result"].(map[string]any)
		result["data"] = map[string]any{"text": "normalized"}
		result["code"] = "FILTERED"
		return map[string]any{"result": result}, nil
	}))
	t.Cleanup(func() { api.RemoveFilter(beforeName); api.RemoveFilter(afterName) })
	name := registerTestAction(t, map[string]any{
		"url": "http://example.invalid", "filters": []any{beforeName, afterName},
	})
	result, err := api.CallBy(name, map[string]any{"prompt": "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if receivedMethod != "PUT" || receivedHeader != "yes" {
		t.Fatalf("before filter did not update request: %s %q", receivedMethod, receivedHeader)
	}
	data, _ := result.Data.(map[string]any)
	if result.Code != "FILTERED" || data["text"] != "normalized" {
		t.Fatalf("after filter did not normalize result: %#v", result)
	}
}

func TestActionInheritanceAndSharedFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"value":"raw"}`))
	}))
	defer server.Close()
	filterName := t.Name() + ".shared"
	var events []string
	api.RegisterFilter(filterName, testFilter(func(_ context.Context, event string, input map[string]any) (map[string]any, error) {
		events = append(events, event)
		if event == "request" {
			request := input["request"].(map[string]any)
			request["headers"] = map[string]any{"X-Shared": "yes"}
			return map[string]any{"request": request}, nil
		}
		if event == "result" {
			result := input["result"].(map[string]any)
			result["code"] = "NORMALIZED"
			return map[string]any{"result": result}, nil
		}
		return nil, nil
	}))
	parent, child := t.Name()+"/parent", t.Name()+"/child"
	api.RegisterAction(parent, map[string]any{"url": server.URL, "method": "POST", "filters": []any{filterName}, "headers": map[string]any{"X-Parent": "yes"}})
	api.RegisterAction(child, map[string]any{"extends": parent, "timeout": "1m"})
	t.Cleanup(func() { api.RemoveAction(parent); api.RemoveAction(child); api.RemoveFilter(filterName) })
	result, err := api.CallBy(child, map[string]any{"hello": "world"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != "NORMALIZED" || !reflect.DeepEqual(events, []string{"request", "result"}) {
		t.Fatalf("inheritance/filter shorthand failed: %#v events=%v", result, events)
	}
}

func TestActionInheritanceRejectsCyclesAndMissingParents(t *testing.T) {
	a, b := t.Name()+"/a", t.Name()+"/b"
	api.RegisterAction(a, map[string]any{"extends": b})
	api.RegisterAction(b, map[string]any{"extends": a})
	t.Cleanup(func() { api.RemoveAction(a); api.RemoveAction(b) })
	if _, err := api.CallBy(a, nil); err == nil || !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected circular inheritance error, got %v", err)
	}
	api.RegisterAction(a, map[string]any{"extends": t.Name() + "/missing"})
	if _, err := api.CallBy(a, nil); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing parent error, got %v", err)
	}
}

func TestAbstractActionCannotBeCalled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	parent, child := t.Name()+"/parent", t.Name()+"/child"
	api.RegisterAction(parent, map[string]any{"abstract": true, "method": "POST"})
	api.RegisterAction(child, map[string]any{"extends": parent, "url": server.URL})
	t.Cleanup(func() { api.RemoveAction(parent); api.RemoveAction(child) })
	if _, err := api.CallBy(parent, nil); err == nil || !strings.Contains(err.Error(), "abstract") {
		t.Fatalf("expected abstract Action error, got %v", err)
	}
	if result, err := api.CallBy(child, nil); err != nil || !result.Ok {
		t.Fatalf("a child of an abstract Action must be callable: result=%#v err=%v", result, err)
	}
}

func TestStreamFilterTransformsAndDropsChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("keep\ndrop\n"))
	}))
	defer server.Close()
	filterName := t.Name() + ".after"
	var events []string
	api.RegisterFilter(filterName, testFilter(func(_ context.Context, event string, input map[string]any) (map[string]any, error) {
		events = append(events, event)
		if event == "request" {
			return map[string]any{"state": map[string]any{"requested": true}}, nil
		}
		if event == "chunk" {
			state, _ := input["state"].(map[string]any)
			if state["requested"] != true {
				t.Fatal("request filter state was not preserved for the response stream")
			}
			chunk := strings.ReplaceAll(string(input["chunk"].([]byte)), "drop\n", "")
			return map[string]any{"chunk": strings.ToUpper(chunk), "state": map[string]any{"requested": true, "seen": true}}, nil
		}
		if event == "done" {
			state, _ := input["state"].(map[string]any)
			if state["seen"] != true {
				t.Fatal("filter state was not preserved for this call")
			}
		}
		return nil, nil
	}))
	t.Cleanup(func() { api.RemoveFilter(filterName) })
	name := registerTestAction(t, map[string]any{
		"url": server.URL, "method": "GET",
		"filters": []any{filterName},
	})
	var got bytes.Buffer
	result, err := api.CallBy(name, nil, &api.CallOptions{Stream: &api.StreamOptions{
		OnData: func(data []byte) error { _, _ = got.Write(data); return nil },
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ok || got.String() != "KEEP\n" || !reflect.DeepEqual(events, []string{"request", "headers", "chunk", "done"}) {
		t.Fatalf("unexpected filtered stream: ok=%v body=%q events=%v", result.Ok, got.String(), events)
	}
}
