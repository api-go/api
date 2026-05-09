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

## ⏱ 性能基准测试 (Benchmark)

使用 `go test -bench=. ./...` 评估框架调用阶段的开销。
> **基准**: Intel Core i9-9980HK 2.40GHz
*   `BenchmarkCallEngineLogic-16`：约 **245 ns/op**, **2 allocs/op**。
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
最后测试日期：2026-05-08
状态：PASS
