package api

import (
	"unsafe"

	"apigo.cc/go/cast"
	"apigo.cc/go/safe"
)

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
	Url             string
	Method          string
	headers         map[string]string
	Payload         any
	wipeableBuffers [][]byte // 追踪需要安全擦除的敏感缓冲区
}

// GetHeader 获取指定 Header 的值
func (r *HttpRequest) GetHeader(key string) string {
	if r == nil || r.headers == nil {
		return ""
	}
	return r.headers[key]
}

// Close 物理覆盖并清除所有关联的敏感缓冲区，确保内存中不再留存明文
func (r *HttpRequest) Close() {
	if r == nil {
		return
	}
	// 擦除缓冲区
	for _, buffer := range r.wipeableBuffers {
		safe.ZeroMemory(buffer)
	}
	r.wipeableBuffers = nil
}

// SetHeader 提供无感知的安全 Header 设置功能。支持传入多个参数进行自动拼接。
// 如果参数中包含 *safe.SafeBuf, *safe.SecretPlaintext 或 []byte (标记为敏感)，
// 整个生成的 Header 缓冲区都将被注册用于后置物理擦除。
// 安全的拼接与转换（如 safe.Concat, safe.Base64）建议使用 safe 包提供的返回 *safe.SafeBuf 的方法。
func (r *HttpRequest) SetHeader(key string, values ...any) {
	if len(values) == 0 {
		return
	}

	if r.headers == nil {
		r.headers = make(map[string]string)
	}

	// 1. 计算总长度并识别是否有敏感数据
	totalLen := 0
	isSensitive := false
	for _, v := range values {
		switch t := v.(type) {
		case string:
			totalLen += len(t)
		case []byte:
			totalLen += len(t)
			isSensitive = true
		case *safe.SafeBuf:
			if t == nil {
				continue
			}
			p := t.Open()
			totalLen += len(p.Data)
			p.Close()
			isSensitive = true
		case *safe.SecretPlaintext:
			if t == nil {
				continue
			}
			totalLen += len(t.Data)
			isSensitive = true
		default:
			s := cast.String(v)
			totalLen += len(s)
		}
	}

	// 2. 分配单一缓冲区进行拼接
	buf := make([]byte, totalLen)
	pos := 0
	for _, v := range values {
		switch t := v.(type) {
		case string:
			pos += copy(buf[pos:], t)
		case []byte:
			pos += copy(buf[pos:], t)
		case *safe.SafeBuf:
			if t == nil {
				continue
			}
			p := t.Open()
			pos += copy(buf[pos:], p.Data)
			p.Close()
		case *safe.SecretPlaintext:
			if t == nil {
				continue
			}
			pos += copy(buf[pos:], t.Data)
		default:
			pos += copy(buf[pos:], cast.String(v))
		}
	}

	// 3. 映射到 headers 并注册清理
	r.headers[key] = unsafe.String(&buf[0], len(buf))
	if isSensitive {
		r.wipeableBuffers = append(r.wipeableBuffers, buf)
	}
}

// Result 定义 API 调用的标准返回结果
type Result struct {
	StatusCode int
	Status     string
	Headers    map[string]string
	Data       any
}
