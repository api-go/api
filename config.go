package api

import (
	"errors"
	"reflect"
	"strings"
	"sync"

	"apigo.cc/go/cast"
	"apigo.cc/go/config"
	"apigo.cc/go/crypto"
	"apigo.cc/go/encoding"
	"apigo.cc/go/safe"
)

var confAES *crypto.Symmetric

func init() {
	crypto.OnSetDefaultAES(func(aes *crypto.Symmetric) {
		confAES = aes
	})
}

var configMutex sync.RWMutex

// GlobalConfigs 存储整棵配置树
var GlobalConfigs = map[string]any{}

// SetEncryptKeys 允许设置自定义的配置加解密密钥
func SetEncryptKeys(key, iv []byte) {
	crypto.SetDefaultAES(key, iv)
}

// SetConfig 允许在内存中动态添加或覆盖配置，适用于从数据库或知识库加载配置的场景。
// 它会自动扫描 map 中的字符串，识别并解密以 "**" 开头的加密内容并转换为 SafeBuf。
func SetConfig(name string, conf map[string]any) {
	configMutex.Lock()
	defer configMutex.Unlock()

	// 1. 扫描并自动解密敏感数据
	decryptMapWithPrefix(conf)
	// 注意：解密出的 SafeBufs 是常驻内存的配置，由 GlobalConfigs 持有，不需要立即关闭

	// 2. 合并到全局树
	if GlobalConfigs[name] == nil {
		GlobalConfigs[name] = conf
	} else if dst, ok := GlobalConfigs[name].(map[string]any); ok {
		MergeMap(dst, conf)
	}
}

// ReplaceConfig replaces one configuration root. It is intended for dynamic
// stores such as DocDB where removed fields must disappear immediately.
func ReplaceConfig(name string, conf map[string]any) {
	configMutex.Lock()
	defer configMutex.Unlock()
	if old, ok := GlobalConfigs[name].(map[string]any); ok {
		closeSecrets(old)
	}
	copyConf := cloneMap(conf)
	decryptMapWithPrefix(copyConf)
	GlobalConfigs[name] = copyConf
}

// Encrypt encrypts plaintext for storage in API configuration.
func Encrypt(plaintext string) (string, error) {
	if confAES == nil {
		return "", errors.New("default AES is not initialized")
	}
	ciphertext, err := confAES.EncryptBytes([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return "**" + encoding.URLBase64(ciphertext), nil
}

// AddConfig 是 SetConfig 的兼容性别名
func AddConfig(name string, conf map[string]any) { SetConfig(name, conf) }

// Load 加载指定的配置文件并合并到 GlobalConfigs
func Load(name string) error {
	if name == "" {
		name = "api"
	}
	var conf map[string]any
	err := config.Load(&conf, name)
	if err != nil {
		return err
	}

	SetConfig(name, conf)
	return nil
}

// GetActionConfig 获取某个动作经过层级合并后的完整配置，返回配置图和需要手动关闭的 SafeBuf 列表
func GetActionConfig(actionName string) (map[string]any, []*safe.SafeBuf) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	parts := strings.Split(actionName, ".")
	res := map[string]any{}

	// 1. 获取 api 根节点
	curr, ok := GlobalConfigs["api"].(map[string]any)
	if !ok {
		// 尝试直接从根开始找
		curr = GlobalConfigs
	}

	// 2. 逐级导航并合并
	for _, part := range parts {
		next, ok := curr[part].(map[string]any)
		if !ok {
			// 尝试在 actions 目录下寻找 (可选约定)
			if actions, ok := curr["actions"].(map[string]any); ok {
				next, ok = actions[part].(map[string]any)
			}
		}

		if next != nil {
			MergeMap(res, next)
			curr = next
		} else {
			break
		}
	}

	return res, nil
}

// fill 注入配置到 Action，非破坏性，仅注入零值
func fill(action any, actionConfig map[string]any) {
	if action == nil || actionConfig == nil {
		return
	}

	if ga, ok := action.(*GenericAction); ok {
		// 对于通用动作，将配置合并到 payload map 中（仅合并 payload 不存在的 key）
		for k, v := range actionConfig {
			if k == "url" || k == "method" || k == "signer" || k == "format" || k == "headers" || k == "timeout" || k == "transport" || k == "websocket" {
				continue
			}
			if _, exists := ga.payload[k]; !exists {
				ga.payload[k] = v
			}
		}
		return
	}

	v := reflect.ValueOf(action)
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			if !f.CanSet() || !f.IsZero() {
				continue
			}

			fieldName := t.Field(i).Name
			if val, ok := findConfigValue(actionConfig, fieldName); ok {
				// 如果目标是 string 但值是 SafeBuf，默认不注入以防泄露，除非手动处理
				if f.Kind() == reflect.String {
					if _, isSafe := val.(*safe.SafeBuf); isSafe {
						continue
					}
				}
				cast.Convert(f.Addr().Interface(), val)
			}
		}
	} else if v.Kind() == reflect.Map {
		for _, key := range v.MapKeys() {
			kv := v.MapIndex(key)
			if isZero(kv) {
				if val, ok := findConfigValue(actionConfig, cast.String(key.Interface())); ok {
					v.SetMapIndex(key, reflect.ValueOf(val))
				}
			}
		}
	}
}

func findConfigValue(m map[string]any, key string) (any, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	lk := strings.ToLower(key)
	for k, v := range m {
		if strings.ToLower(k) == lk {
			return v, true
		}
	}
	return nil, false
}

func isZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr, reflect.Map, reflect.Slice:
		return v.IsNil()
	default:
		return v.IsZero()
	}
}

// MergeMap 深度合并两个 map
func MergeMap(dst, src map[string]any) {
	for k, v := range src {
		if k == "actions" || k == "raw" {
			continue
		}
		if v == nil {
			continue
		}

		if srcMap, ok := v.(map[string]any); ok {
			dstVal, ok := dst[k]
			var dstMap map[string]any
			if ok {
				dstMap, _ = dstVal.(map[string]any)
			}
			if dstMap == nil {
				dstMap = make(map[string]any)
				dst[k] = dstMap
			}
			MergeMap(dstMap, srcMap)
		} else {
			dst[k] = v
		}
	}
}

func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		switch t := v.(type) {
		case map[string]any:
			dst[k] = cloneMap(t)
		case []any:
			dst[k] = append([]any(nil), t...)
		case []string:
			dst[k] = append([]string(nil), t...)
		default:
			dst[k] = v
		}
	}
	return dst
}

func closeSecrets(m map[string]any) {
	for _, v := range m {
		switch t := v.(type) {
		case *safe.SafeBuf:
			t.Close()
		case map[string]any:
			closeSecrets(t)
		}
	}
}

func decryptMapWithPrefix(m map[string]any) []*safe.SafeBuf {
	var safeBufs []*safe.SafeBuf
	for k, v := range m {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "**") && confAES != nil {
			raw := s[2:]
			b64, err := encoding.UnURLBase64(raw)
			if err == nil && len(b64) > 0 {
				if dec, err := confAES.DecryptBytes(b64); err == nil {
					sb := safe.NewSafeBufAndErase(dec)
					m[k] = sb
					safeBufs = append(safeBufs, sb)
					continue
				}
			}
		} else if subMap, ok := v.(map[string]any); ok {
			safeBufs = append(safeBufs, decryptMapWithPrefix(subMap)...)
		}
	}
	return safeBufs
}
