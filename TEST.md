# TEST

本项目通过纯粹无状态的本地截获测试来验证核心逻辑的准确性。所有的测试用例都被设计为在不依赖外部网络和具体云厂商配置的情境下运行，确保了高并发下的稳定性和测试隔离性。

## 🧪 测试覆盖

### 1. 配置继承与合并 (`TestConfigInheritance`)
验证 `mockSvc.subSvc.doAction` 多级 Key 能够正确执行深层合并与覆盖，且支持读取嵌套在 `actions` 中的分支结构。

### 2. 标准签名器验证 (`TestBuiltinSigners`)
脱离网络环境，验证 `Basic` 与 `Bearer` 两种内置认证机制的请求头注入结果是否符合预期。

### 3. 全链路模拟调用与非破坏性注入 (`TestFullCallFlow`)
利用 `httptest.Server` 在本地建立截获端点：
*   **非破坏性注入 (Fill)**：验证 `AppId`, `Secret` 等预设零值被成功回填，而带有明确赋值的 `Name` 等字段未被覆盖。
*   **占位符处理移交**：验证 `{{.service}}` 等 URL 模板变量能够被自定义的 Mock 签名器接管、解析并替换。
*   **序列化与协议透传**：验证自动由 `http` 包进行 Payload 解析后的 JSON 数据流与 HTTP Method 和 Authentication Header 一致性。

## 🛡️ 安全性验证 (Security Audit)

### 4. 内存安全闭环与后置擦除 (`TestSafeConfigDecryption`, `TestFillSafeGuard`)
验证敏感数据在内存中的全生命周期保护：
*   **SafeBuf 自动转换**：所有解密后的配置字段强制转换为 `safe.SafeBuf`，有效防止内存 Dump。
*   **后置字符串擦除**：验证通过 `unsafe.String` 传递给 `http.Header` 的敏感字符串，在 `httpReq.Close()` 后被物理覆盖（内容不再是原始密钥）。
*   **注入保护 (Guard)**：敏感数据禁止自动注入 Action 的 `string` 字段，杜绝无意中的内存留存。
*   **自动生命周期回收**：`api.Call` 结束时通过 `defer` 机制强制调用 `Close()` 擦除本次请求的所有明文密钥副本及 Header 缓冲区。

### 5. 动态策略与统一结果 (`TestTokenAndOverridePolicy`, `TestResultRulesAndConfigOverride`)
验证 Action 内部 Token、控制字段黑名单、密文字段不可覆盖、URL/Method 临时覆盖，以及 `codeFields/successCodes/errorFields` 对统一结果的判断。

### 6. 流式调用 (`TestStreamPreservesChunkOrder`)
使用 `ManualDo` 验证流式 Body 的分块顺序、同步回调、完成事件和统一结果状态。

### 7. 动态 Action 继承与统一 Filter 管线

验证 `extends` 深度继承、父 Action 热解析、循环和缺失父项报错，以及 `filters: []` 同时收到请求与响应事件。JavaScript Filter 的中文流片段按 UTF-8 字符串传递。

### 8. JavaScript 业务结果投影 (`TestProjectDataResult`, `TestActionUsesInheritedDataResult`)

验证 `resultMode: data` 仅向 JavaScript 返回业务数据与 `ok/code/error`，不会泄露 HTTP Headers；同时验证子 Action 能继承父 Action 的投影策略。

### 9. Action YAML (`TestParseActionYAML`, `TestPatchActionTokensPreservesDocument`, `TestConvertActionJSONToYAML`)

验证 Action YAML 的对象解析、非对象拒绝、保留注释与字段顺序的 Token 补丁，以及不注入供应商默认值的 JSON 到 YAML 转换。

## ⏱ 性能基准测试 (Benchmark)

使用 `go test -bench=. ./...` 评估框架调用阶段的开销。
> **基准**: Darwin / Apple M3 Max
*   `BenchmarkCallEngineLogic-16`：约 **115.5 ns/op**, **80 B/op**, **2 allocs/op**。
该指标证明引擎的参数合并、注入及校验流程具有极高的运行效率和极小的内存逃逸。

## 🚀 运行测试

```bash
cd api
# 运行业务测试，禁用缓存
go test -v -count 1 ./...
# 运行性能测试
go test -bench=. ./...
```

---
最后测试日期：2026-08-19
状态：PASS
