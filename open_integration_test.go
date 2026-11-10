package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/api-go/api"
	"github.com/gorilla/websocket"
)

func TestManagedOpenRetainsHandshakeRejectionDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Reason", "denied")
		w.WriteHeader(403)
		_, _ = io.WriteString(w, `{"error":"denied"}`)
	}))
	defer server.Close()
	name := registerTestAction(t, map[string]any{"url": server.URL, "transport": "websocket"})
	result, err := api.CallBy(name, "hello")
	if err == nil || result.Ok || result.StatusCode != 403 || result.Headers["X-Reason"] != "denied" {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	detail, ok := result.ErrorDetail.(map[string]any)
	if !ok || detail["error"] != "denied" {
		t.Fatalf("detail=%#v", result.ErrorDetail)
	}
}

func TestManagedOpenReconnectsAfterInitiatingContextCancellation(t *testing.T) {
	var connections atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		connections.Add(1)
		for {
			kind, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if conn.WriteMessage(kind, data) != nil {
				return
			}
		}
	}))
	defer server.Close()
	name := registerTestAction(t, map[string]any{"url": server.URL, "transport": "websocket", "websocket": map[string]any{"connectionID": "shared", "responseMode": "first", "idleTimeout": time.Second}})
	first, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	result, err := api.CallBy(name, "one", &api.CallOptions{Context: first})
	if err != nil || !result.Ok {
		t.Fatalf("first=%#v err=%v", result, err)
	}
	cancelFirst()
	second, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	result, err = api.CallBy(name, "two", &api.CallOptions{Context: second})
	if err != nil || !result.Ok {
		t.Fatalf("second=%#v err=%v", result, err)
	}
	if connections.Load() != 2 {
		t.Fatalf("connections=%d", connections.Load())
	}
}
