package api

import (
	"fmt"
	"strings"

	"apigo.cc/go/cast"
	"apigo.cc/go/http"
)

// Call 是调度引擎的入口
func Call[T any](action Action) (*T, error) {
	// 1. 获取并合并配置
	actionConfig := map[string]any{}
	if ca, ok := action.(ConfigurableAction); ok {
		MergeMap(actionConfig, ca.Config())
	}
	MergeMap(actionConfig, GetActionConfig(action.ActionName()))

	// 2. 业务自校验
	if va, ok := action.(ValidatableAction); ok {
		if err := va.Validate(); err != nil {
			return nil, fmt.Errorf("action validation failed: %w", err)
		}
	}

	// 3. 注入配置到 Action (Payload)
	fill(action, actionConfig)

	// 4. 确定 Method 和 URL
	method := "POST"
	if ma, ok := action.(MethodAction); ok {
		method = ma.GetMethod()
	}

	url := ""
	if ua, ok := action.(URLAction); ok {
		url = ua.GetURL()
	}
	if url == "" {
		url = cast.String(actionConfig["url"])
		if url == "" {
			url = cast.String(actionConfig["host"])
			if url != "" && !strings.Contains(url, "://") {
				url = "https://" + url
			}
		}
	}

	// 5. 构建请求描述对象
	httpReq := &HttpRequest{
		Url:     url,
		Method:  strings.ToUpper(method),
		Headers: make(map[string]string),
		Payload: action,
	}

	// 合并默认 Header
	if headers, ok := actionConfig["headers"].(map[string]any); ok {
		for k, v := range headers {
			httpReq.Headers[k] = cast.String(v)
		}
	}

	// 6. 执行签名 (签名器需负责处理 URL 中的动态变量)
	if sa, ok := action.(SignerAction); ok {
		if err := sign(sa.SignerName(), httpReq, actionConfig); err != nil {
			return nil, fmt.Errorf("sign failed: %w", err)
		}
	}

	// 7. 发起底层 HTTP 调用
	timeout := cast.Duration(actionConfig["timeout"])
	client := http.NewClient(timeout)

	res := client.Do(httpReq.Method, httpReq.Url, httpReq.Payload, headerSlice(httpReq.Headers)...)
	if res.Error != nil {
		return nil, res.Error
	}

	// 8. 解析响应
	var response T
	if err := res.To(&response); err != nil {
		var temp any
		_ = res.To(&temp)
		cast.Convert(&response, temp)
	}

	return &response, nil
}

func headerSlice(headers map[string]string) []string {
	res := make([]string, 0, len(headers)*2)
	for k, v := range headers {
		res = append(res, k, v)
	}
	return res
}
