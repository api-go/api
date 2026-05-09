package api

import (
	"reflect"
	"strings"
	"sync"

	"apigo.cc/go/cast"
	"apigo.cc/go/config"
	"apigo.cc/go/crypto"
	"apigo.cc/go/encoding"
	"apigo.cc/go/safe"
)

var confAes, _ = crypto.NewAESGCMAndEraseKey([]byte("?GQ$0K0GgLdO=f+~L68PLm$uhKr4'=tV"), []byte("VFs7@sK61cj^f?HZ"))
var keysOnce = sync.Once{}
var configMutex sync.RWMutex

// GlobalConfigs 存储整棵配置树
var GlobalConfigs = map[string]any{}

// SetEncryptKeys 允许设置自定义的配置加解密密钥
func SetEncryptKeys(key, iv []byte) {
	keysOnce.Do(func() {
		if confAes != nil {
			confAes.Close()
		}
		confAes, _ = crypto.NewAESGCMAndEraseKey(key, iv)
	})
}

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

	configMutex.Lock()
	defer configMutex.Unlock()

	// 合并到全局树
	MergeMap(GlobalConfigs, conf)
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
		return res, nil
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

	safeBufs := decryptMap(res)
	return res, safeBufs
}

// fill 注入配置到 Action，非破坏性，仅注入零值
func fill(action any, actionConfig map[string]any) {
	if action == nil || actionConfig == nil {
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

func decryptMap(m map[string]any) []*safe.SafeBuf {
	var safeBufs []*safe.SafeBuf
	for k, v := range m {
		if s, ok := v.(string); ok {
			var b64 []byte
			var err error
			if b64, err = encoding.UnUrlBase64FromString(s); err != nil {
				b64, err = encoding.UnBase64FromString(s)
			}

			if err == nil && len(b64) > 0 {
				if dec, err := confAes.DecryptBytes(b64); err == nil {
					sb := safe.NewSafeBufAndErase(dec)
					m[k] = sb
					safeBufs = append(safeBufs, sb)
					continue
				}
			}
		} else if subMap, ok := v.(map[string]any); ok {
			safeBufs = append(safeBufs, decryptMap(subMap)...)
		}
	}
	return safeBufs
}
