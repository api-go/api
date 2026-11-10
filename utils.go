package api

import (
	"fmt"
	"sort"
	"strings"

	"github.com/api-go/cast"
	"github.com/api-go/safe"
)

// GetStringOrSafe 从配置中安全地获取字符串值。如果是 SafeBuf，则返回明文副本及对应的清理函数
func GetStringOrSafe(v any) (string, func()) {
	if v == nil {
		return "", func() {}
	}
	if sb, ok := v.(*safe.SafeBuf); ok {
		p := sb.Open()
		return p.String(), p.Close
	}
	return cast.String(v), func() {}
}

// GetBytesOrSafe 从配置中安全地获取字节切片。如果是 SafeBuf，则返回明文副本及对应的清理函数
func GetBytesOrSafe(v any) ([]byte, func()) {
	if v == nil {
		return nil, func() {}
	}
	if sb, ok := v.(*safe.SafeBuf); ok {
		p := sb.Open()
		return p.Data, p.Close
	}
	return []byte(cast.String(v)), func() {}
}

// ExportGoStructs 根据全局配置生成 Go 结构体代码
func ExportGoStructs(packageName string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("package %s\n\n", packageName))

	sb.WriteString("// API Services and Actions\n")
	keys := make([]string, 0, len(GlobalConfigs))
	for k := range GlobalConfigs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("// Service: %s\n", k))
	}

	return sb.String()
}

// GetManifest 返回 API 的描述信息，方便 AI 阅读
func GetManifest() map[string]any {
	manifest := map[string]any{}
	for name, cfg := range GlobalConfigs {
		if cfgMap, ok := cfg.(map[string]any); ok {
			manifest[name] = getActionManifest(cfgMap)
		}
	}
	return manifest
}

func getActionManifest(cfgMap map[string]any) map[string]any {
	m := map[string]any{
		"url":    cfgMap["url"],
		"method": cfgMap["method"],
	}
	if actions, ok := cfgMap["actions"].(map[string]any); ok {
		manifestActions := map[string]any{}
		for name, sub := range actions {
			if subMap, ok := sub.(map[string]any); ok {
				manifestActions[name] = getActionManifest(subMap)
			}
		}
		m["actions"] = manifestActions
	}
	return m
}
