package api

import (
	"fmt"
	"sort"
	"strings"
)

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
