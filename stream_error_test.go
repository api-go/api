package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"apigo.cc/go/api"
)

func TestStreamingHTTPFailureRunsResponseFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400) }))
	defer server.Close()
	filterName := t.Name() + ".session"
	var events []string
	api.RegisterFilter(filterName, testFilter(func(_ context.Context, event string, input map[string]any) (map[string]any, error) {
		events = append(events, event)
		if event == "request" {
			return map[string]any{"state": map[string]any{"parent": "previous"}}, nil
		}
		if event == "result" {
			if input["state"].(map[string]any)["parent"] != "previous" {
				t.Fatal("request state lost")
			}
			r := input["result"].(map[string]any)
			r["data"] = map[string]any{"localHistoryRequired": true}
			return map[string]any{"result": r}, nil
		}
		return nil, nil
	}))
	t.Cleanup(func() { api.RemoveFilter(filterName) })
	name := registerTestAction(t, map[string]any{"url": server.URL, "method": "GET", "filters": []any{filterName}})
	r, err := api.CallBy(name, nil, &api.CallOptions{Stream: &api.StreamOptions{OnData: func([]byte) error { t.Error("failed request emitted stream content"); return nil }, OnDone: func(map[string]any) error { t.Error("failure emitted successful stream completion"); return nil }}})
	if err == nil || r.Ok || r.StatusCode != 400 {
		t.Fatalf("failure was hidden: %#v %v", r, err)
	}
	if !reflect.DeepEqual(events, []string{"request", "result"}) {
		t.Fatalf("failure bypassed response lifecycle: %v", events)
	}
	if r.Data.(map[string]any)["localHistoryRequired"] != true {
		t.Fatal("response filter result lost")
	}
}
