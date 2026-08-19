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
3.  **发起调用**：使用 `api.Call(action, options)`，统一获得 `Result`。

## 🛠 接口说明

*   `Action`：核心标识接口，定义动作名称（如 `tencent.sms.smsPackagesStatistics`）。
*   `SignerAction`：指定签名算法名称（如 `tc3`）。
*   `ConfigurableAction`：提供硬编码的默认参数或元数据。
*   `URLAction` / `MethodAction`：动态指定 Endpoint 和 HTTP 方法。
*   `ValidatableAction`：业务参数自校验。
*   `CallOptions`：提供内部 Token、超时、临时配置覆盖、Logger 与流式回调。
*   `Result`：统一返回 `ok/statusCode/headers/data/code/error`，业务响应保留在 `data`。
*   `RegisterAction` / `RemoveAction`：动态注册和热更新数据驱动的 Action。
*   `RegisterJSSigner` / `RemoveSigner`：管理 JavaScript Signer。
*   `RegisterJSFilter` / `RemoveFilter`：管理可复用的 JavaScript 请求/响应 Filter。
*   `ParseActionYAML` / `PatchActionTokens` / `ConvertActionJSONToYAML`：无状态解析、保注释补丁和旧格式转换；不内置任何供应商 Action。

## 🔒 安全性 (Ultimate Memory Safety)

*   **内置解密**：只识别 `**URLBase64(AESGCM)` 配置密文。
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

result, err := api.Call(&MySmsAction{Limit: 10}, &api.CallOptions{
    Token: "internal-token",
    Config: map[string]any{"path": "/v1/messages"},
})
```

动态 Action 可以在调用时覆盖非敏感配置：

```go
api.RegisterAction("openai", map[string]any{
    "url": "http://127.0.0.1:8001/v1/chat/completions",
    "method": "POST",
    "signer": "none",
    "tokens": []string{"internal-token"},
})

result, err := api.CallBy("openai", payload, &api.CallOptions{
    Token: "internal-token",
    Config: map[string]any{"path": "/v1/embeddings"},
})
```

`tokens` 未配置或为空时无需 Token。`tokens/enabled/test/logging` 与原配置中的密文字段不能通过 `options.config` 覆盖。流式调用使用 `go/http.ManualDo`，`OnHeaders` 与 `OnDone` 接收 lower-camel map，`OnData` 接收原始字节块。

Action 只使用 `filters: ["name"]` 这一种形式。每个 Filter 都会收到前置 `request` 以及后置 `headers/chunk/result/done` 事件，并自行根据 `event` 决定是否处理。流式 Filter 应通过输入/输出的 `state` 保存单次调用状态，不能假设 HTTP chunk 与 SSE 或 JSONL 消息边界一致。JavaScript Filter 的 `chunk` 是 UTF-8 字符串；Go Filter 仍接收原始 `[]byte`，二进制响应不应挂载文本型 JavaScript Filter。

动态 Action 可通过 `extends` 继承另一个 Action；父配置先合并，子配置深度覆盖。继承在每次调用时解析，因此父 Action 热更新会立即作用于子 Action。循环继承或不存在的父 Action 会直接返回错误。

面向 JavaScript 低代码调用时，Action 可配置 `resultMode: data`，使 `api.Call` 直接返回业务 `data`，并保留顶层 `ok/code/error`，避免 HTTP 状态和响应头干扰应用逻辑。该配置支持 `extends` 继承，只影响 JavaScript 导出；Go 的 `Call` 与 `CallBy` 始终返回完整 `Result`。

---
更多详情请参阅 [TEST.md](./TEST.md) 和 [CHANGELOG.md](./CHANGELOG.md)。
