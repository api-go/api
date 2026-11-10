package api_test

import (
	"testing"

	"github.com/api-go/api"
)

func TestModelConfiguredUsesEffectiveActionWithoutCallingProvider(t *testing.T) {
	const parent, child = "ready-fixture-parent", "ready-fixture-child"
	api.RegisterAction(parent, map[string]any{"abstract": true, "url": "http://127.0.0.1:1", "model": "fixture"})
	api.RegisterAction(child, map[string]any{"extends": parent})
	t.Cleanup(func() { api.RemoveAction(child); api.RemoveAction(parent) })
	if api.ModelConfigured(parent, "") || !api.ModelConfigured(child, "") || api.ModelConfigured("not-registered-fixture", "fixture") {
		t.Fatal("inheritance or callable configuration was not respected")
	}
	api.RegisterAction(child, map[string]any{"extends": parent, "enabled": false})
	if api.ModelConfigured(child, "") {
		t.Fatal("disabled model accepted")
	}
	api.RegisterAction(child, map[string]any{"url": "http://127.0.0.1:1"})
	if api.ModelConfigured(child, "") || !api.ModelConfigured(child, "override") {
		t.Fatal("missing model or explicit override was not respected")
	}
	api.RemoveAction(child)
	if api.ModelConfigured(child, "override") {
		t.Fatal("removed configuration remains ready")
	}
}
