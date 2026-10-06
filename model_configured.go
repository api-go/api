package api

import (
	"strings"

	"apigo.cc/go/cast"
)

// ModelConfigured checks a dynamic model Action without making a request or
// exposing its configuration. Use the same inheritance resolver as CallBy.
func ModelConfigured(name, model string) bool {
	config, err := resolveGenericActionConfig(strings.TrimSpace(name), map[string]bool{})
	if err != nil || cast.Bool(config["abstract"]) || config["enabled"] == false {
		return false
	}
	if strings.TrimSpace(cast.String(config["url"])) == "" && strings.TrimSpace(cast.String(config["host"])) == "" {
		return false
	}
	if strings.TrimSpace(model) != "" || strings.TrimSpace(cast.String(config["model"])) != "" || strings.TrimSpace(cast.String(config["defaultModel"])) != "" {
		return true
	}
	models, _ := config["models"].(map[string]any)
	return len(models) > 0
}
