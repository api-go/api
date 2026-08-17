package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apigo.cc/go/api"
	"apigo.cc/go/cast"
	"apigo.cc/go/encoding"
)

// =========================================================================
// 1. 配置继承与合并测试
// =========================================================================
func TestConfigInheritance(t *testing.T) {
	api.GlobalConfigs = map[string]any{} // 确保环境干净
	api.GlobalConfigs["api"] = map[string]any{
		"mockSvc": map[string]any{
			"url":    "https://api.mock.com",
			"signer": "mockSigner",
			"config": map[string]any{
				"secretId": "globalId",
			},
			"actions": map[string]any{
				"subSvc": map[string]any{
					"config": map[string]any{
						"service": "subSvc",
					},
					"actions": map[string]any{
						"doAction": map[string]any{
							"url": "/subSvc/doAction",
							"config": map[string]any{
								"secretId": "overrideId",
							},
						},
					},
				},
			},
		},
	}

	cfg, _ := api.GetActionConfig("mockSvc.subSvc.doAction")
	if cfg == nil {
		t.Fatal("config not found")
	}

	if cfg["url"] != "/subSvc/doAction" {
		t.Errorf("expected url /subSvc/doAction, got %v", cfg["url"])
	}
	if cfg["signer"] != "mockSigner" {
		t.Errorf("expected signer mockSigner, got %v", cfg["signer"])
	}

	actionConfig, ok := cfg["config"].(map[string]any)
	if !ok {
		t.Fatal("action config not found or invalid type")
	}
	if actionConfig["secretId"] != "overrideId" {
		t.Errorf("expected secretId overrideId, got %v", actionConfig["secretId"])
	}
	if actionConfig["service"] != "subSvc" {
		t.Errorf("expected service subSvc, got %v", actionConfig["service"])
	}
}

// =========================================================================
// 2. 签名器逻辑测试
// =========================================================================
func TestBuiltinSigners(t *testing.T) {
	// 测试 Basic 签名器
	req := &api.HttpRequest{}
	config := map[string]any{
		"username": "admin",
		"password": "123",
	}
	signer := api.GetSigner("basic")
	if signer == nil {
		t.Fatal("basic signer not found")
	}
	if err := signer.Sign(req, config); err != nil {
		t.Fatal(err)
	}
	expected := "Basic " + encoding.Base64([]byte("admin:123"))
	if req.GetHeader("Authorization") != expected {
		t.Errorf("expected %s, got %s", expected, req.GetHeader("Authorization"))
	}

	// 测试 Bearer 签名器
	req = &api.HttpRequest{}
	config = map[string]any{
		"token": "secret-token",
	}
	signer = api.GetSigner("bearer")
	if signer == nil {
		t.Fatal("bearer signer not found")
	}
	signer.Sign(req, config)
	if req.GetHeader("Authorization") != "Bearer secret-token" {
		t.Errorf("expected Bearer secret-token, got %s", req.GetHeader("Authorization"))
	}
}

// =========================================================================
// 3. 完整调用与参数注入测试 (无状态网络环境)
// =========================================================================

// 模拟外部专有签名器
type mockExtSigner struct{}

func (s *mockExtSigner) Sign(req *api.HttpRequest, rawConfig map[string]any) error {
	// 获取动态配置信息
	service, _ := rawConfig["service"].(string)

	// 签名器内部处理 URL 动态重写
	if strings.Contains(req.Url, "{{.service}}") {
		req.Url = strings.ReplaceAll(req.Url, "{{.service}}", service)
	}

	// 添加模拟签名
	req.SetHeader("Authorization", "Mock-Signature-Pass")
	return nil
}

func init() {
	api.RegisterSigner("mockExtSigner", &mockExtSigner{})
}

// 模拟 Action 定义
type MockCallAction struct {
	Name   string
	Age    int
	AppId  string // 预期由引擎的 Fill 机制自动注入
	Secret string // 预期由引擎的 Fill 机制自动注入
}

func (MockCallAction) ActionName() string { return "mockPlatform.mockAction" }
func (MockCallAction) SignerName() string { return "mockExtSigner" }
func (MockCallAction) Config() map[string]any {
	return map[string]any{
		"service": "mockService",
	}
}

type MockResponse struct {
	Status string
	Echo   string
}

func TestFullCallFlow(t *testing.T) {
	// 启动本地截获 HTTP 的 Mock Server
	var receivedURL string
	var receivedAuth string
	var receivedMethod string
	var receivedBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		receivedAuth = r.Header.Get("Authorization")
		receivedMethod = r.Method
		receivedBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success", "echo": "Hello from mock server"}`))
	}))
	defer ts.Close()

	// 初始化隔离的全局配置
	api.GlobalConfigs = map[string]any{}
	api.GlobalConfigs["api"] = map[string]any{
		"mockPlatform": map[string]any{
			"url":   "{{.service}}.example.com", // 带占位符的 host (会被 signer 替换)
			"appId": "inject-app-id",
			"actions": map[string]any{
				"mockAction": map[string]any{
					"url":    ts.URL + "?host={{.service}}.example.com", // 为了让流量指向 ts，我们在 query 模拟
					"secret": "inject-secret",
				},
			},
		},
	}

	action := &MockCallAction{
		Name: "Alice",
		Age:  30,
	}

	// 执行调度调用
	resp, err := api.Call(action)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	// 1. 验证 Fill 参数注入
	if action.AppId != "inject-app-id" {
		t.Errorf("expected injected AppId inject-app-id, got %s", action.AppId)
	}
	if action.Secret != "inject-secret" {
		t.Errorf("expected injected Secret inject-secret, got %s", action.Secret)
	}
	if action.Name != "Alice" {
		t.Errorf("Fill should not overwrite existing value, expected Alice, got %s", action.Name)
	}

	// 2. 验证响应绑定
	var response MockResponse
	cast.Convert(&response, resp.Data)
	if response.Status != "success" || response.Echo != "Hello from mock server" {
		t.Errorf("Response bind failed, got %+v", resp)
	}

	// 3. 验证 HTTP 透传数据
	if receivedMethod != "POST" {
		t.Errorf("expected POST method, got %s", receivedMethod)
	}
	if receivedAuth != "Mock-Signature-Pass" {
		t.Errorf("expected signature header, got %s", receivedAuth)
	}

	// 验证 Signer 中 URL 占位符已被替换
	if !strings.Contains(receivedURL, "host=mockService.example.com") {
		t.Errorf("expected Signer to replace URL placeholder, got URL: %s", receivedURL)
	}

	// 验证 Payload 的 JSON 序列化 (由底层 apigo.cc/go/http 处理)
	var bodyMap map[string]any
	json.Unmarshal(receivedBody, &bodyMap)
	if bodyMap["name"] != "Alice" || bodyMap["appId"] != "inject-app-id" {
		t.Errorf("Payload body serialization incorrect: %s", string(receivedBody))
	}
}

// 性能测试：验证纯逻辑开销 (无网络)
func BenchmarkCallEngineLogic(b *testing.B) {
	api.GlobalConfigs = map[string]any{
		"api": map[string]any{
			"benchPlatform": map[string]any{
				"appId": "bench-id",
			},
		},
	}

	actionConfig, _ := api.GetActionConfig("benchPlatform")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// 仅测试引擎准备阶段的开销 (合并、校验、注入)，不走网络
		api.MergeMap(map[string]any{}, actionConfig)
		// 这里由于 fill 仅限于 package 内部，我们不能直接调 fill()，
		// 但我们实际上想衡量的是整个准备流程。
		// 由于 Call 会触发网络，这里只压测 GetActionConfig (最核心合并解析逻辑)
		_, _ = api.GetActionConfig("mockPlatform.mockAction")
	}
}
