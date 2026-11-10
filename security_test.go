package api

import (
	"strings"
	"testing"

	"github.com/api-go/encoding"
	"github.com/api-go/safe"
)

func TestSafeConfigDecryption(t *testing.T) {
	// 1. 准备测试环境
	key := []byte("12345678123456781234567812345678")
	iv := []byte("123456781234")
	SetEncryptKeys(key, iv)

	plaintext := "my-secret-password"
	encoded, err := Encrypt(plaintext)
	if err != nil || !strings.HasPrefix(encoded, "**") {
		t.Fatalf("Encrypt() = %q, %v", encoded, err)
	}
	ciphertext, _ := confAES.EncryptBytes([]byte(plaintext))
	b64 := "**" + encoding.URLBase64(ciphertext)

	GlobalConfigs = map[string]any{}
	SetConfig("api", map[string]any{
		"testSvc": map[string]any{
			"password": b64,
			"username": "admin",
		},
	})

	// 2. 测试获取配置
	cfg, sbs := GetActionConfig("testSvc")
	if len(sbs) != 0 {
		t.Fatalf("expected persistent config secrets, got %d temporary buffers", len(sbs))
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
	err = signer.Sign(req, cfg)
	if err != nil {
		t.Fatal(err)
	}

	expectedAuth := "Basic " + encoding.Base64([]byte("admin:"+plaintext))
	if req.GetHeader("Authorization") != expectedAuth {
		t.Errorf("expected %s, got %s", expectedAuth, req.GetHeader("Authorization"))
	}

	// 4. 测试生命周期管理 (清理)
	authStr := req.GetHeader("Authorization")
	req.Close()

	// 验证请求 Header 在调用结束后被擦除，常驻配置则仍可用于下一次调用。
	if authStr == expectedAuth {
		t.Error("Authorization header should be modified/erased after Close")
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
