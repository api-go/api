# CHANGELOG

## [v1.5.16] - 2026-10-06

- 发布模型配置就绪状态检查接口

`ModelConfigured` uses the Action inheritance resolver without calling a provider or exposing configuration. It rejects missing, abstract, disabled, and incomplete model Actions. Covered by `TestModelConfiguredUsesEffectiveActionWithoutCallingProvider`.

## v1.5.15 (2026-09-13)
- **托管 WebSocket Transport**: `Call` 支持 Action 驱动的 WebSocket 连接复用、单在途租约、Filter 终态、空闲回收、Action 更新失效和握手错误归一化，并自动导出到既有 JavaScript `api.Call`。

## v1.5.14 (2026-09-12)

- **受保护参数**: 动态 API Action 可用 `protectedInputs` 显式声明允许展开的 payload 路径；引用只在当前低代码调用上下文内、请求 Filter 完成后解析，并参与后续签名和 HTTP 编码。

## v1.5.12 (2026-09-05)
- **依赖对齐**: 补齐 `config`、`file`、`http` 与 `log` 的级联版本，保留 API 包的直接 YAML 依赖。

## v1.5.11 (2026-09-05)
- **依赖对齐**: 升级全部 `apigo.cc/go/*` 依赖至当前已发布版本，保留 API 包的直接 YAML 解析依赖。

## v1.5.9 (2026-08-25)
- **Filter 中间件顺序**: 请求事件按配置正序执行，响应与流式生命周期事件按配置逆序执行，使外层 Filter 能消费内层归一化结果。
- **供应商错误详情**: HTTP 非成功响应保留原始业务体到 `errorDetail`，供上层 Filter 统一错误格式并保留诊断信息。
- **依赖对齐**: 升级 `log` 与 `shell` 到当前稳定补丁版本。

## v1.5.8 (2026-08-19)
- **Action YAML**: 增加无状态解析、保注释 Token 补丁和 JSON 到 YAML 转换能力，不绑定存储或内置第三方服务定义。
- **可选 Timing**: `CallOptions.Timing` 按需返回毫秒级 `total`，流式调用额外返回首个响应数据块的 `firstToken`。
- **JavaScript 结果一致性**: `api.Call` 的完整结果统一投影为 lower-camel Map，避免 Go 结构体字段名泄露到低代码层。
- **依赖对齐**: 升级 `file`、`id`、`rand` 和 `shell` 到当前稳定补丁版本。

## v1.5.7 (2026-08-17)
- **低代码结果投影**: 动态 Action 可通过 `resultMode: data` 让 JavaScript `api.Call` 直接返回业务数据，并补齐 `ok/code/error`。
- **继承一致性**: 结果投影读取完整的 Action 继承配置，子 Action 无需重复声明。
- **兼容边界**: Go `Call`/`CallBy` 与未配置该选项的 JavaScript Action 均保持原有完整 `Result` 行为。

## v1.5.6 (2026-08-17)
- **统一调用结果**: `Call` 与 `CallBy` 返回 `ok/statusCode/headers/data/code/error`，支持响应错误字段和成功码规则。
- **调用选项**: 新增 Token 验证、超时、配置覆盖与敏感/控制字段覆盖保护。
- **流式能力**: 基于 `go/http.ManualDo` 增加同步 Headers/Data/Done 回调，支持 JS 低代码向下游响应透传。
- **动态管理**: 增加 Action 与 JavaScript Signer 的注册、替换和移除能力。
- **统一 Filter 管线**: Action 仅配置有序 `filters: []`；同一 Filter 实例按 `request/headers/chunk/result/done` 事件处理调用全生命周期，并提供逐调用隔离状态。
- **安全配置**: 密文格式收敛为 `**URLBase64(AESGCM)`，新增无状态 `Encrypt`。
- **结构化审计**: 增加 API 调用日志；正文默认关闭，开启后仍执行文本、数组和深度裁剪。
- **依赖对齐**: 升级 `cast/config/crypto/encoding/http/log/safe` 到当前稳定补丁版本。

## v1.5.5 (2026-06-21)
- **错误堆栈重构**:
  - `Call` 函数的返回错误使用 `jsmod.MakeError` 动态包裹以保留调用堆栈。
- **依赖对齐**:
  - 升级依赖 `jsmod` 至 `v1.5.3`, `cast` 至 `v1.5.3`, `rand` 至 `v1.5.3`, `encoding` 至 `v1.5.4`, `shell` 至 `v1.5.3`, `safe` 至 `v1.5.2`, `id` 至 `v1.5.4`, `crypto` 至 `v1.5.3`, `file` 至 `v1.5.5`, `config` 至 `v1.5.3`, `log` 至 `v1.5.8`, `http` 至 `v1.5.3`。

## v1.5.4 (2026-06-11)
- **依赖对齐**:
  - 对齐基础设施依赖到 v1.5.x。

## v1.5.3 (2026-06-10)
- **优化**:
  - 对齐 JS 导出为 PascalCase 格式。

## v1.5.2 (2026-06-10)
- **修复**:
  - 对齐 encoding 模块 v1.5.2。

## v1.5.1 (2026-06-03)
- **依赖对齐**:
  - 对齐 Tag v1.5.0。

## v1.5.0 (2026-05-30)
- **基础设施升级**:
  - 与全局 v1.5.0 基础设施对齐。

## v1.4.0 (2026-05-30)
- **重大重构**: 引入数据驱动的 API 编排模式。
- **新增**: `ActionRegistry` 支持注册 Go 结构体或 JSON 定义作为 API 模板。
- **新增**: `CallBy(name, payload)` 动态调用入口，自动处理模板实例化与动态参数合并。
- **新增**: `SetConfig` 支持内存配置热更新，并自动识别 `**` 前缀的加密字符串为 `SafeBuf`。
- **新增**: 支持 `form` 和 `multipart` 请求格式。
- **新增**: 跨语言签名支持，提供 `SetJSRunner` 钩子允许执行 JS 签名逻辑。
- **JSMOD**: 注册 `call`, `setConfig`, `registerAction`, `registerSigner` 到 `jsmod`。

## v1.3.3 (2026-05-30)

### Security
- **Frictionless Memory Safety**: introduced `HttpRequest.SetHeader(key, values...)` with automatic secret extraction and buffer tracking.
- **Automated Lifecycle**: `api.Call` now automatically "Opens" all `SafeBuf` configuration values before calling Signers, and explicitly closes them via defer.
- **Paradigm Signers**: refactored built-in signers to use naive, declarative syntax (`safe.Base64(safe.Concat(...))`) which leverages `SafeBuf` for encrypted intermediate states, preventing GC reliance.
- **Ultimate Protection**: implemented post-hoc string erasure using `unsafe.String` over controlled buffers managed by a private `wipeableBuffers` list.
- **Infrastructure Alignment**: aligned dependencies with `go/safe` v1.0.6, `go/http` v1.0.11, and `go/config` v1.0.7.

## v1.0.2 (2026-05-09)

### Changed
*   **移除第三方依赖**: 移除了对 `jsontag` 模块的依赖，统一使用标准库及自有基础设施对齐，增强了模块的独立性与长期稳定性。

## v1.0.1 (2026-05-08)

### Refactoring & Testing
*   **彻底无状态化**：剥离真实外部服务依赖与网络调用测试。
*   **本地全链路拦截**：使用 `httptest.Server` 重塑测试用例，从配置合并、填充 (`Fill`)、自定义签名器到 HTTP 序列化全程闭环。
*   **性能提升证明**：新增 Benchmark，确认核心调度在纳秒级别（245 ns/op），0 反射滥用，极低逃逸。
*   **清理**：合并所有冗余散落的测试文件至单一 `api_test.go`。

## v1.0.0 (2026-05-08)

### Features
*   **核心引擎**：实现基于接口驱动的 API 调用调度器。
*   **配置系统**：支持多级层级合并、自动解密、并发安全保护。
*   **注入引擎**：实现非破坏性参数注入（仅注入零值字段）。
*   **标准签名器**：内置 `basic`, `bearer` 认证支持。
*   **AI 增强**：提供 `GetManifest` 导出功能。

### Refactoring
*   彻底移除字符串模板解析引擎，转向无状态设计。
*   将具体签名器（如 TC3）剥离至具体业务层。
