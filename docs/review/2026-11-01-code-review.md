# 代码深审报告 — pitaya · 2026-11-01(代码评审席)

范围:全仓 `pkg/**` + `e2e` + 工具脚本;方法:requesting-code-review 流程(① 安全静态扫描 → ② 隐藏 bug 猎捕 → ③ 质量门基线对比)。只读扫描为主;P0/P1 修复以工作区补丁形式落地并独立复核。

> **本仓同任务存在并行审计席**(见 `docs/审计-文档一致性.md` 复审记录,及 git log 中 `docs: fix 26 doc-source drift items` / `fix: share mutex in ReqInFlight value copies` / `fix: set repl prettyJSON package var` 等异席 commit)。代码修复以并行席工作区补丁为准,本席独立复核并验证;本席 commit 仅含本报告与 `todo.md`,不代其提交未审阅批次。

## 一、安全静态扫描(常规向量)

| 检查项 | 结论 | 证据 |
|---|---|---|
| 硬编码密钥/凭据 | 未发现 | 全仓检索命中仅配置字段名、默认空值、测试占位 |
| shell 注入 | 未发现 | `pkg/modules/binary.go:67` 为运维配置的启动命令(operator-controlled,非客户端输入);`pkg/helpers` 内为测试辅助 |
| SQL 拼接 | N/A | 全仓无 SQL(持久层不在本仓) |
| eval / unsafe / plugin / reflect 滥用 | 未发现 | `unsafe`/`plugin.Open`/`reflect.New` 命中仅测试 mock |
| 路径穿越 | 未发现 | `repl/file.go:42` 路径来自 REPL 运维命令行参数,非客户端输入 |
| 反序列化(yaml/gob/xml) | 未发现 | 配置用 viper;网络消息为 JSON/protobuf + 自研 packet/message 编码 |
| **未限定解压(zip-bomb)** | **发现 → 已修(P1)** | `pkg/util/compression/compression.go` InflateData 原为无界 `io.ReadAll` |
| 协议解码内存安全 | 通过 | `pomelo_packet_decoder.go` ParseHeader 上限 `MaxPacketSize=1<<24` 且校验 type;message Decode 路由长度有界检查 |

## 二、隐藏 bug(按严重度)

### P1(2 项,已修 — 工作区)

1. **未限定解压(zip-bomb)** — `compression.go` `InflateData` 无界读入,恶意/损坏压缩流可撑爆内存。修复:`MaxInflatedSize = 1<<24`(`compression.go:14`)+ `io.LimitReader(zr, MaxInflatedSize+1)`(`:36`)+ 超限报错(`:40`);新增 `TestCompressionInflateZipBombRejected`(`compression_test.go:61`)。
2. **`parallelGetter` 按值返回导致 `sync.Mutex` 拷贝** — `etcd_service_discovery.go` `newParallelGetter` 原返回值类型,`go vet` 报 `return copies lock value`(clean HEAD `:469`)。修复:改返回 `*parallelGetter`(`etcd_service_discovery.go:457`)。

### P2(2 项,已修 — 工作区)

3. **`TestEncode` fixture 版本依赖失败** — `mustGzipData` 39 字节负载在部分 zlib 版本下不可压缩,与 golden 不符。修复:`message_test.go:21` 改 `strings.Repeat("blabla", 20)`(120 字节,可稳定压缩)。
4. **`TestCompressionDeflate` golden 精确比对版本依赖** — 修复:`compression_test.go` 改往返断言(round-trip)替代 golden 精确比对。

### P3(3 项)

5. **`message` ID varint 解码无 8 字节上限** — `message_encoder.go:150-159`:varint 循环以 `len(data)` 为界,Go 定义移位 ≥ 宽度结果为 0,故内存安全;但未强制 64 位(8 字节)varint 上限,畸形报文可让 ID 解码吞掉整段负载。建议超过 8 个 continuation 字节即返回 `ErrInvalidMessage`。**未修**(协议边界行为,记 `todo.md`)。
6. **测试 unkeyed composite literal(vet)** — `pomelo_packet_decoder_test.go:36-37` 3 处 `packet.Packet` 字面量用未命名字段。已随并行席工作区修正(`pkg/conn/packet/packet.go` 加字段名)。
7. **`gofmt` 格式漂移** — clean HEAD `gofmt -l` 命中约 9 文件(ws_acceptor.go / packet.go / models.go / binary.go / binary_windows.go / binding_storage.go / remote.go / session.go / compression.go)。已随并行席工作区修正(工作区 `gofmt -l` 现为 0)。

## 三、质量门基线对比

| 门 | 清洁基线 `6492aa3` | 工作区(修复后) |
|---|---|---|
| `go build ./...` | 绿 | 绿 |
| `go vet ./...` | 4 诊断:lock 拷贝(P1,`:469`)+ unkeyed 3 处(P3,`pomelo_packet_decoder_test.go:36-37`) | **0 输出** |
| `gofmt -l .` | 约 9 文件 | **0 文件** |
| `go test ./...`(非 e2e) | 31 包 ok;2 用例失败(fixture 版本依赖) | **34 包全绿** |
| `e2e` | 环境失败:缺预构建 `examples/testing/server` | 同。非缺陷:`Makefile:13` 的 `TESTABLE_PACKAGES` 显式排除 `e2e`;`make e2e-test` 先经 `ensure-testing-bin` 构建二进制(`Makefile:97`)再跑 |

## 四、总账

- **发现:7**(P0=0,P1=2,P2=2,P3=3)
- **已修:6**(P1×2,P2×2,P3×2 —— 均在并行席工作区,本席已验证绿)
- **剩余:1**(P3 varint 8 字节上限,记 `todo.md`)
- 安全静态扫描:常规向量 CLEAN,唯一安全相关缺口(zip-bomb)已修。
- 质量门:build/vet/gofmt 全绿;测试 34 包全绿;e2e 失败为环境/门禁设计,非缺陷。

## 五、commit 清单

**本席(报告 + todo):** 见落地 commit。

**并行审计席(同任务,已观察):**
- `09af627` docs: fix 26 doc-source drift items
- `2d66ab6` fix: share mutex in ReqInFlight value copies
- `ac82e6a` fix: set repl prettyJSON package var
- `7f564e8` docs: add architecture diagram svg
- `4ed4d34` docs: drop retired goreportcard badge, embed architecture diagram
- P1/P2/P3 代码修复(compression / etcd_service_discovery / message_test / compression_test / 等)现于其工作区待提交;本席复核:build、vet、gofmt、非 e2e 测试全绿。
