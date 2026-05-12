package api

import (
	"testing"

	"apigo.cc/go/encoding"
	"apigo.cc/go/safe"
)

func TestSafeConfigDecryption(t *testing.T) {
	// 1. 准备测试环境
	key := []byte("12345678123456781234567812345678")
	iv := []byte("123456781234")
	SetEncryptKeys(key, iv)

	plaintext := "my-secret-password"
	ciphertext, _ := confAES.EncryptBytes([]byte(plaintext))
	b64 := encoding.Base64ToString(ciphertext)

	GlobalConfigs = map[string]any{
		"api": map[string]any{
			"testSvc": map[string]any{
				"password": b64,
				"username": "admin",
			},
		},
	}

	// 2. 测试获取配置
	cfg, sbs := GetActionConfig("testSvc")
	if len(sbs) != 1 {
		t.Fatalf("expected 1 SafeBuf, got %d", len(sbs))
	}

	sb, ok := cfg["password"].(*safe.SafeBuf)
	if !ok {
		t.Fatal("password should be *safe.SafeBuf")
	}

	p := sb.Open()
	if p.String() != plaintext {
		t.Errorf("expected %s, got %s", plaintext, p.String())
	}
	p.Close()

	// 3. 测试签名器使用 SafeBuf
	req := &HttpRequest{}
	signer := GetSigner("basic")
	err := signer.Sign(req, cfg)
	if err != nil {
		t.Fatal(err)
	}

	expectedAuth := "Basic " + encoding.Base64ToString([]byte("admin:"+plaintext))
	if req.GetHeader("Authorization") != expectedAuth {
		t.Errorf("expected %s, got %s", expectedAuth, req.GetHeader("Authorization"))
	}

	// 4. 测试生命周期管理 (清理)
	authStr := req.GetHeader("Authorization")
	req.Close()

	for _, sb := range sbs {
		sb.Close()
	}

	// 验证 Authorization Header 已被擦除 (内容不再是原始数据)
	if authStr == expectedAuth {
		t.Error("Authorization header should be modified/erased after Close")
	}

	// 再次尝试 Open 应该失败或得到空 (取决于 SafeBuf 实现，通常 Close 后内容被擦除)
	p2 := sb.Open()
	if p2.String() == plaintext && len(plaintext) > 0 {
		t.Error("SafeBuf should be cleared after Close")
	}
}

func TestFillSafeGuard(t *testing.T) {
	type SecretAction struct {
		Password string
		AppId    string
	}

	sb := safe.NewSafeBuf([]byte("secret"))
	defer sb.Close()

	config := map[string]any{
		"Password": sb,
		"AppId":    "my-app",
	}

	action := &SecretAction{}
	fill(action, config)

	if action.AppId != "my-app" {
		t.Errorf("AppId should be filled, got %s", action.AppId)
	}

	if action.Password != "" {
		t.Error("Sensitive SafeBuf should NOT be filled into string field automatically")
	}
}
