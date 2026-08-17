package api

import (
	"fmt"
	"reflect"
	"strings"

	"apigo.cc/go/cast"
	"apigo.cc/go/http"
	"apigo.cc/go/safe"
)

// Call 是调度引擎的入口
func legacyCall[T any](action Action) (*T, error) {
	// 1. 获取并合并配置
	actionConfig, safeBufs := GetActionConfig(action.ActionName())
	defer func() {
		for _, sb := range safeBufs {
			sb.Close()
		}
	}()

	if ca, ok := action.(ConfigurableAction); ok {
		MergeMap(actionConfig, ca.Config())
	}

	// 1.5 预处理配置：自动 Open 所有 SafeBuf 变为临时的 SecretPlaintext
	// 这样 Signer 可以直接使用这些值，且在 defer 中会被自动回收
	var openedSecrets []*safe.SecretPlaintext
	preprocessSecrets(actionConfig, &openedSecrets)
	defer func() {
		for _, secret := range openedSecrets {
			secret.Close()
		}
	}()

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
		Payload: action,
	}
	defer httpReq.Close()

	// 合并默认 Header
	if headers, ok := actionConfig["headers"].(map[string]any); ok {
		for k, v := range headers {
			httpReq.SetHeader(k, v)
		}
	}

	// 6. 执行签名 (签名器需负责处理 URL 中的动态变量)
	if sa, ok := action.(SignerAction); ok {
		if err := sign(sa.SignerName(), httpReq, actionConfig); err != nil {
			return nil, fmt.Errorf("sign failed: %w", err)
		}
	}

	// 7. 处理请求格式 (Form/Multipart)
	payload := httpReq.Payload
	if fa, ok := action.(FormatAction); ok {
		switch strings.ToLower(fa.GetFormat()) {
		case "form":
			var f http.Form
			cast.Convert(&f, payload)
			payload = f
		case "multipart":
			var m http.Multipart
			cast.Convert(&m, payload)
			payload = m
		}
	}

	// 8. 发起底层 HTTP 调用
	timeout := cast.Duration(actionConfig["timeout"])
	client := http.NewClient(timeout)

	res := client.Do(httpReq.Method, httpReq.Url, payload, headerSlice(httpReq)...)
	if res.Error != nil {
		return nil, res.Error
	}

	// 9. 解析响应
	var response T
	if err := res.To(&response); err != nil {
		var temp any
		_ = res.To(&temp)
		cast.Convert(&response, temp)
	}

	return &response, nil
}

// CallBy 通过动作名称和动态 Payload 发起调用
func legacyCallBy[T any](name string, payload any) (*T, error) {
	tmpl, ok := actionRegistry[name]
	if !ok {
		return nil, fmt.Errorf("action not found: %s", name)
	}

	var action Action
	if t, ok := tmpl.(reflect.Type); ok {
		// Go 结构体模式
		inst := reflect.New(t).Interface()
		if payload != nil {
			cast.Convert(inst, payload)
		}
		action = inst.(Action)
	} else if ga, ok := tmpl.(*GenericAction); ok {
		// JSON 定义模式 (深拷贝模板以防并发冲突)
		newGA := &GenericAction{
			name:    ga.name,
			config:  cloneMap(ga.config),
			payload: make(map[string]any),
		}
		// 复制模板 Payload
		for k, v := range ga.payload {
			newGA.payload[k] = v
		}
		// 合并动态 Payload
		if payload != nil {
			if m, ok := payload.(map[string]any); ok {
				for k, v := range m {
					newGA.payload[k] = v
				}
			} else {
				cast.Convert(&newGA.payload, payload)
			}
		}
		action = newGA
	}

	return legacyCall[T](action)
}

func preprocessSecrets(m map[string]any, opened *[]*safe.SecretPlaintext) {
	for k, v := range m {
		if sb, ok := v.(*safe.SafeBuf); ok {
			secret := sb.Open()
			m[k] = secret
			*opened = append(*opened, secret)
		} else if subMap, ok := v.(map[string]any); ok {
			preprocessSecrets(subMap, opened)
		}
	}
}

func headerSlice(req *HttpRequest) []string {
	res := make([]string, 0, len(req.headers)*2)
	for k, v := range req.headers {
		res = append(res, k, v)
	}
	return res
}
