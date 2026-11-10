package api

import (
	"testing"

	"github.com/api-go/jsmod"
)

func TestProjectDataResult(t *testing.T) {
	result := &Result{Ok: true, StatusCode: 200, Headers: map[string]string{"X-Test": "ignored"}, Data: map[string]any{"result": "hello"}, Timing: map[string]any{"unit": "ms", "total": int64(12)}}
	projected := projectDataResult(result).(map[string]any)
	if projected["ok"] != true || projected["result"] != "hello" || projected["code"] != "" || projected["error"] != "" {
		t.Fatalf("unexpected projected result: %#v", projected)
	}
	if _, exists := projected["headers"]; exists {
		t.Fatalf("transport headers leaked into projected result: %#v", projected)
	}
	if projected["timing"].(map[string]any)["total"] != int64(12) {
		t.Fatalf("timing was not projected: %#v", projected)
	}
}

func TestResultMapUsesLowerCamelKeys(t *testing.T) {
	mapped := resultMap(&Result{Ok: true, StatusCode: 200, Timing: map[string]any{"unit": "ms"}})
	if mapped["ok"] != true || mapped["statusCode"] != 200 || mapped["timing"] == nil {
		t.Fatalf("unexpected result map: %#v", mapped)
	}
	if _, exists := mapped["Ok"]; exists {
		t.Fatalf("Go field name leaked into result map: %#v", mapped)
	}
}

func TestActionUsesInheritedDataResult(t *testing.T) {
	parent, child := t.Name()+".parent", t.Name()+".child"
	RegisterAction(parent, map[string]any{"abstract": true, "resultMode": "data"})
	RegisterAction(child, map[string]any{"extends": parent, "url": "http://127.0.0.1"})
	t.Cleanup(func() { RemoveAction(child); RemoveAction(parent) })
	if !actionUsesDataResult(child) {
		t.Fatal("child Action did not inherit resultMode=data")
	}
}

func TestJSFilterRegistrationIsUnsafe(t *testing.T) {
	module := jsmod.GetModules()["api"]
	for _, name := range []string{"SetConfig", "RegisterAction", "RegisterSigner"} {
		if module == nil || !module.UnsafeList[name] {
			t.Fatalf("JS API mutation %s must be blocked in safe mode", name)
		}
	}
}
