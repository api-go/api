# CHANGELOG

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
