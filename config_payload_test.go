package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"apigo.cc/go/api"
	"apigo.cc/go/cast"
)

func TestDynamicConfigurationDoesNotBecomeProviderPayload(t *testing.T) {
	bodies := make(chan map[string]any, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		body, err := cast.FromJSON[map[string]any](string(raw))
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/quote" || r.URL.Query().Get("route") != "contract" || r.Header.Get("X-Fixture") != "retained" {
			t.Error("transport configuration was not retained")
		}
		bodies <- body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"accepted","fee_cents":1000}`))
	}))
	defer server.Close()
	parent, child := t.Name()+"Parent", t.Name()+"Child"
	api.RegisterAction(parent, map[string]any{
		"url": server.URL, "path": "/quote", "method": "POST", "format": "json",
		"query": map[string]any{"route": "contract"}, "headers": map[string]any{"X-Fixture": "retained"},
		"name": "configuration-name", "title": "internal title", "summary": "internal summary", "protocol": "http",
		"abstract": true, "enabled": true, "tokens": []string{"fixture-token"},
		"creation": map[string]any{"childFields": []string{"url"}}, "requestSchema": map[string]any{"type": "object"},
		"responseSchema": map[string]any{"type": "object"}, "test": map[string]any{"request": map[string]any{"test": "internal fixture"}},
		"response": map[string]any{"codeFields": []string{"code"}, "successCodes": []string{"accepted"}},
		"logging":  false, "protectedInputs": []string{}, "resultMode": "data", "mock": map[string]any{"enabled": false},
		"warehouse_id": "warehouse", "payload": map[string]any{"currency": "CNY"},
	})
	api.RegisterAction(child, map[string]any{"extends": parent})
	defer api.RemoveAction(parent)
	defer api.RemoveAction(child)
	for _, explicit := range []bool{false, true} {
		payload := map[string]any{"destination": "HZ", "weight_grams": 500}
		if explicit {
			payload["name"], payload["test"] = "provider business name", "provider business field"
		}
		result, err := api.CallBy(child, payload, &api.CallOptions{Token: "fixture-token"})
		if err != nil || !result.Ok {
			t.Fatalf("call failed: %+v %v", result, err)
		}
		body := <-bodies
		wantKeys := 4
		if explicit {
			wantKeys = 6
			if body["name"] != payload["name"] || body["test"] != payload["test"] {
				t.Fatal("explicit provider fields were stripped")
			}
		}
		if len(body) != wantKeys || body["destination"] != "HZ" || cast.Int(body["weight_grams"]) != 500 || body["warehouse_id"] != "warehouse" || body["currency"] != "CNY" {
			t.Fatalf("provider received configuration or lost business defaults: keys=%v", body)
		}
	}
}
