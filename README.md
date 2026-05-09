# @go/api

`@go/api` 是一个极致精简、接口驱动、AI 友好的 API 调用引擎。它旨在消除对接第三方服务（如腾讯云、阿里云、OpenAI 等）时的繁琐 SDK 依赖和硬编码摩擦。

## 🎯 设计哲学

*   **数据驱动 (Data-Driven)**：一切皆数据，通过强类型 Action 结构体描述请求。
*   **无状态 (Stateless)**：核心库不绑定任何具体的云服务实现，仅提供标准协议支持。
*   **AI 友好 (AI-First)**：极其简单的结构体定义，消除了幻觉，让 AI 能够精准生成调用代码。
*   **非破坏性注入**：支持从配置自动回填缺失参数（如 AppId, SecretId），但不覆盖显式设置的值。

## 📦 安装

```bash
go get apigo.cc/go/api
```

## 💡 核心流程

1.  **定义 Action**：实现 `Action` 接口（及可选的 `SignerAction`, `ConfigurableAction` 等）。
2.  **配置授权**：在 `api.yml` 或环境变量中配置密钥。
3.  **发起调用**：使用 `api.Call[Response](&action)`。

## 🛠 接口说明

*   `Action`：核心标识接口，定义动作名称（如 `tencent.sms.smsPackagesStatistics`）。
*   `SignerAction`：指定签名算法名称（如 `tc3`）。
*   `ConfigurableAction`：提供硬编码的默认参数或元数据。
*   `URLAction` / `MethodAction`：动态指定 Endpoint 和 HTTP 方法。
*   `ValidatableAction`：业务参数自校验。

## 🔒 安全性 (Ultimate Memory Safety)

*   **内置解密**：支持自动识别并解密配置中的 AES 加密内容。
*   **内存保护**：敏感配置解密后以 `safe.SafeBuf` 形式存储，防止内存 Dump 泄露。
- **防止字符串泄露**：通过 `unsafe.String` 零拷贝技术，确保敏感 Header（如 Authorization）在调用结束后可被物理擦除，彻底解决 Go 字符串不可变性导致的堆泄露问题。
- **全生命周期闭环**：`api.Call` 结束后自动触发 `httpReq.Close()`，对所有中间缓冲区进行 `ZeroMemory` 随机覆盖。
- **安全辅助函数**：内置 `SetBasicAuth`, `SetBearerAuth` 等工具，强制执行无拼接的内存安全逻辑。

## 🧪 示例

```go
type MySmsAction struct {
    Limit int
    AppId string // 自动从配置注入
}
func (MySmsAction) ActionName() string { return "tencent.sms.smsPackagesStatistics" }
func (MySmsAction) SignerName() string { return "tc3" }

resp, err := api.Call[MyResponse](&MySmsAction{Limit: 10})
```

---
更多详情请参阅 [TEST.md](./TEST.md) 和 [CHANGELOG.md](./CHANGELOG.md)。
