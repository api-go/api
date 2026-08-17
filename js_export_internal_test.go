package api

import "testing"

func TestProjectDataResult(t *testing.T) {
	result := &Result{Ok: true, StatusCode: 200, Headers: map[string]string{"X-Test": "ignored"}, Data: map[string]any{"result": "hello"}}
	projected := projectDataResult(result).(map[string]any)
	if projected["ok"] != true || projected["result"] != "hello" || projected["code"] != "" || projected["error"] != "" {
		t.Fatalf("unexpected projected result: %#v", projected)
	}
	if _, exists := projected["headers"]; exists {
		t.Fatalf("transport headers leaked into projected result: %#v", projected)
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
