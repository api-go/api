package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/api-go/api"
	"github.com/api-go/log"
)

func TestInputPreviewIsOptInAndUnicodeBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer server.Close()
	const name = "input-preview-test"
	api.RegisterAction(name, map[string]any{"url": server.URL, "method": "POST", "format": "json"})
	defer api.RemoveAction(name)
	for _, enabled := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "preview.log")
		logger := log.NewLogger(log.Config{Level: "info", File: path})
		result, err := api.CallBy(name, map[string]any{"input": []string{strings.Repeat("中", 20) + "不要记录后文", "第二项正文"}}, &api.CallOptions{Logger: logger, InputPreview: enabled})
		if err != nil || !result.Ok {
			t.Fatalf("call: %v %v", result, err)
		}
		var restored bytes.Buffer
		err = log.QueryFile(context.Background(), path+".gz", &restored, log.QueryOptions{Raw: true})
		data := restored.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if strings.Contains(text, "InputPreview") != enabled || enabled && !strings.Contains(text, strings.Repeat("中", 20)+"…") || strings.Contains(text, "不要记录后文") || strings.Contains(text, "第二项正文") {
			t.Fatalf("incorrect bounded preview: %s", text)
		}
	}
}
