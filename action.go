package api

// Action 是所有接口的基础标识接口
type Action interface {
	ActionName() string // 例如: "tencent.sms.send"
}

// SignerAction 定义需要签名的动作
type SignerAction interface {
	SignerName() string // 例如: "tc3"
}

// ConfigurableAction 定义可以提供默认硬编码配置的动作
type ConfigurableAction interface {
	Config() map[string]any
}

// URLAction 定义显式指定 URL 的动作
type URLAction interface {
	GetURL() string
}

// MethodAction 定义显式指定方法的动作
type MethodAction interface {
	GetMethod() string // 例如: "GET"
}

// ValidatableAction 定义支持自我校验的动作
type ValidatableAction interface {
	Validate() error
}

// HttpRequest 内部使用的请求描述结构，供 Signer 使用
type HttpRequest struct {
	Url     string
	Method  string
	Headers map[string]string
	Payload any
}

// Result 定义 API 调用的标准返回结果
type Result struct {
	StatusCode int
	Status     string
	Headers    map[string]string
	Data       any
}
