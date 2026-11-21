# 代码深审报告 — pitaya · 2026-11-01(最终汇总)

范围:全仓 `pkg/**` + `e2e`;方法:requesting-code-review 流程(① 安全静态扫描 → ② 隐藏 bug 猎捕 → ③ 质量门基线对比)。所有缺陷条目均经本席**逐条对照源码验证**(file:line 证据)后才落账;专项 agent 报告中无法独立复现的条目一律标注,不晋升为缺陷。

> 本报告为同任务并行审计席初版报告(见 git log `docs: add code review report and todo list`)的最终汇总版:并入后续深审新发现的 P0/P1 修复与回归测试。并行席已落工作区的修复(压缩上限 / lock 拷贝 / fixture / vet / gofmt)由本席复核验证。

## 一、P0(1 项,已修)

1. **消息解码越界 panic(远程可触发)** — `pkg/conn/message/message_encoder.go:200`:route 长度分支在 `rl := data[offset]` 之前无 `offset >= size` 守卫(原 `offset > size` 检查晚于索引)。Request 报文带 ID 无 route 长度字节(线上输入 `[]byte{0x00, 0x01}`)即令 `data[2]` 越界 panic,任意客户端可使服务端崩溃。
   **修复**:`if offset >= size { return nil, ErrInvalidMessage }`(同 :200,`b8a3e23`)。
   **回归测试**:`TestDecodeEmptyRouteLengthDoesNotPanic`(`pkg/conn/message/message_test.go`)。

## 二、P1(11 项,已修)

2. **zip-bomb 无界解压** — `compression.go` `InflateData` 原为无界 `io.ReadAll`,客户端可控 gzipMask(`message_encoder.go` 解码路径)可撑爆内存。
   **修复**:`MaxInflatedSize = 1<<24`(`compression.go:14`,与协议 MaxPacketSize 对齐)+ `io.LimitReader(zr, MaxInflatedSize+1)`(`:36`)+ 超限报错(`:40`;`731295a`)。测试:`TestCompressionInflateZipBombRejected`(`pkg/util/compression/compression_test.go`)。
3. **`HasRequestsInFlight` 无锁读 map** — `session.go:1002` 读 `requestsInFlight.m` 不持锁,`SetRequestInFlight`(`:1012` 起)持 `mu.Lock()` 写 → 并发 map 读写。
   **修复**:RLock + RUnlock(`0e8014e`)。
4. **`uid` 数据竞争** — `session.go:453` `UID()` 无锁读 `s.uid`;`Bind` 路径在 `:517/:524/:542/:563/:574` 无锁写。
   **修复**:`UID()` 加 RLock;各写处加 Lock/Unlock(`Clear` 的 `:945` 原本已持锁,不动;`8846bbd`)。
5. **`GetDataEncoded` 数据竞争** — `session.go:477` 无锁读 `s.encodedData`;`updateEncodedData`(`:338`)持 Lock 写。
   **修复**:RLock + RUnlock(`0e8014e`)。
6. **`session.Close` 非幂等(SessionCount 双扣)** — `session.go:606` 无条件 `SessionCount-1`。重连路径(`Bind` 直接关同 UID 旧 session)与旧连接 agent 心跳超时关闭路径(`handler.go`)会对同一 session 双调 Close。
   **修复**:新增 `closed int32` 字段,`Close` 顶部 `atomic.CompareAndSwapInt32` 守卫(`:609`,`0e8014e`)。`NewSession` 每次新建对象,标志不复用。测试:`TestSessionCloseIdempotent`(`pkg/session/session_test.go`)。
7. **WS acceptor TLS 永远显示未运行** — `ws_acceptor.go` 原仅 plain Listen 路径(`:144`)设 `w.running = true`;`ListenAndServeTLS`(`:150`)漏设 → TLS 部署下 `IsRunning/GetStatus` 永远 false。
   **修复**:TLS 路径补 `w.running = true`(`:168`,`205fb8c`)。
8. **acceptor 监听状态数据竞争(WS + TCP 同构)** — `GetAddr` 读 `listener`(ws `:81` / tcp `:106`)与 `ListenAndServe` 写 `listener`(ws `:124` / tcp `:137`)竞争;`IsRunning` 读 `running`(ws `:70` / tcp `:220`)与写侧竞争;TCP `serve`(`:197`)循环逐轮读 `running/listener` 与 `Stop` 写竞争(`go test -race` 复现)。
   **修复**:两 acceptor 加 `sync.RWMutex`,`GetAddr/IsRunning/Stop/ListenAndServe(TLS)` 全部持锁;TCP `serve` 改为持 RLock 快照 `running/listener` 后再 `Accept`(`205fb8c` + `65ac329`)。
   **附带测试修复**:`tcp_acceptor_test.go` goroutine 与主 goroutine 并发写共享 `err` 变量(`-race` 复现)→ 该错误本未检查,改为丢弃。
9. **空服务器表 `rand.Intn(0)` panic** — `router.go:74`(修复前)`srvList[rnd.Intn(len(srvList))]`:目标类型无在线实例时 len=0 直接 panic,RPC 分发崩溃。
   **修复**:`defaultRoute` 空表返回 nil(`:71`,`e500da7`),`Route` 调用点返回 `constants.ErrNoServersAvailableOfType`(`:96/:105/:114`)。测试:`TestDefaultRouteEmptyServers` / `TestRouteNoServersAvailable`(`pkg/router/router_test.go`)。
10. **nil reply 致 reflect panic** — `rpc.go:69`(修复前)`reflect.TypeOf(reply).Kind()`:reply 为 nil 接口时 `TypeOf` 返回 nil,`.Kind()` panic。
    **修复**:`reply == nil ||` 短路返回 `ErrReplyShouldBePtr`(`01f4395` + 类型化 nil 加固 `a2c87d5`)。测试:`TestDoSendRPCNilReply`(`pkg/rpc_test.go`)。
11. **客户端心跳 interval<=0 panic** — `client.go:315`(修复前)`time.NewTicker` 无守卫,interval<=0 直接 panic(non-positive interval)。server 下发 heartbeat=0 即触发。
    **修复**:`sendHeartbeats` 顶部加 `if interval <= 0 { return }`(`:315`,`3771328`)。测试:`TestSendHeartbeatsNonPositiveInterval`(`pkg/client/client_test.go`)。
12. **`parallelGetter` 按值返回拷贝互斥锁** — `pkg/cluster/etcd_service_discovery.go` `newParallelGetter` 原返回值类型(`go vet` copylocks:goroutine 引用局部量的锁,调用方持副本锁,潜在竞态)。
    **修复**:返回 `*parallelGetter`(`:457`,`c970864`)。

## 三、P2/质量门(已修,并行席落盘、本席复核)

13. **测试 fixture 版本依赖 ×2** — `mustGzipData` 39 字节负载在部分 zlib 版本下不可压缩 → 改 `strings.Repeat("blabla", 20)`(120 字节)并重新生成 `pkg/conn/message/fixtures/test_must_gzip.golden`(`b8a3e23`);`compression_test.go` 改往返断言替代 golden 精确比对。
14. **unkeyed composite literal(vet)** — `pomelo_packet_decoder_test.go` 改命名字段(`8b2c64b`)。
15. **gofmt 漂移 9 文件** — 全部格式化(`8b2c64b`);`gofmt -l` 现为 0。
16. **`-race` 测试自身竞态(agent 包)** — 多个 watcher goroutine 用未同步 bool + 忙轮询 → 改 `atomic.Bool` + 单次接收(`agent_test.go:646-648`,`0e8014e`)。

## 四、P3 与说明项(记录,未改)

17. **varint ID 解码无长度上限**(P3,后经 follow-up 修复)— 原始发现:`message_encoder.go` ID varint 循环以 `len(data)` 为界,无 64 位上限,畸形报文可让 ID 解码吞掉整段负载。**已修复**:varint 收紧至最多 10 字节并拒绝第 10 字节溢出与未终止 varint(`message_encoder.go:158/:169`,`b8a3e23`);随后跟进编解码对称性(10 字节上限 + 注释修正,`bec9511`)与平台无关 `uint64` 承载(`6c3b8a1`)。测试:`TestDecodeRejectsOverflowingMessageIDVarint`、`TestEncodeDecodeFull64BitID`(`pkg/conn/message/message_test.go`)。`todo.md` 已记录闭环。
18. **`GetData` 返回活引用** — `session.go:460` 读已持 RLock(读安全),但返回 map 本体,调用方可无锁变更;改深拷贝属行为变更,记注意项。
19. **`TestAgentWriteChSendWriteError` 顺序依赖** — 依赖包级 `once.Do`(`newAgent`,`agent.go:196`)被先前测试消费;`-run` 单测失败、整包通过。既有测试基建问题,非本次引入。
20. **timer 冗余入队(降级)** — `pkg/timer.go:55` 与 `pkg/timer/timer.go:109` 对同一 timer 双发 `ChCreatedTimer`;`AddTimer` 为按 ID 的 `sync.Map` Store,幂等,无双重执行,仅冗余发送。

## 五、安全静态扫描(常规向量)

| 检查项 | 结论 | 证据 |
|---|---|---|
| 硬编码密钥/凭据 | 未发现 | 全仓检索仅配置字段名、默认空值、测试占位 |
| shell 注入 | 未发现 | `pkg/modules/binary.go` 为运维配置命令(operator-controlled) |
| SQL 拼接 | N/A | 全仓无 SQL |
| eval/unsafe/plugin 滥用 | 未发现 | 命中仅测试 mock |
| 路径穿越 | 未发现 | repl 路径来自 REPL 运维命令行,非客户端输入 |
| 反序列化(yaml/gob/xml) | 未发现 | 配置用 viper;网络消息 JSON/protobuf + 自研编码 |
| zip-bomb | **发现 → 已修(P1 #2)** | 见上 |
| 协议解码内存安全 | P0 越界一处 → 已修;其余通过 | packet 头 `MaxPacketSize` 有界 |

**设计面建议(不改框架架构,记录)**:acceptor 层无连接数上限/限速;gRPC RPC server 无认证拦截器。属部署与架构决策,建议在部署文档或后续 ADR 处理。

## 六、质量门基线对比

| 门 | 清洁基线 | 修复后工作区 |
|---|---|---|
| `go build ./...` | 绿 | **绿** |
| `go vet ./...` | 4 诊断(lock 拷贝 + unkeyed 3 处) | **0 输出** |
| `gofmt -l` | ~9 文件 | **0 文件** |
| `go test ./...`(非 e2e,nats down) | 31 包 ok,2 用例失败(fixture 版本依赖) | **全绿** |
| `e2e` | 环境失败(缺二进制/依赖) | **绿**(101s):`make ensure-testing-bin` 构建二进制 + `examples/testing` `docker compose up -d`(nats/etcd;redis 与宿主 6379 冲突不影响 nats e2e) |
| `go test -race` | 既有失败(见下) | 本席触及全部包绿(session/agent/acceptor/conn/router/client 等);**两处既有失败非本次回归**:① 根包 `app.go:393` 包级全局写 vs `app_test.go` 读(pristine HEAD worktree 复现同一竞态);② cluster 内嵌 NATS 测试与外部 nats:4222 端口互斥(nats_rpc 系测试,环境约束) |

## 七、总账

- **发现:P0=1,P1=11,P2/质量=4,P3/说明=4,设计建议=2**(20 项 + 2 设计建议)
- **已修:P0×1 + P1×11 + P2×4 + P3×1**(#17 varint 经 follow-up commit 闭环);其中 #12-16 由并行审计席落工作区,本席逐条复核验证
- **剩余:P3 说明项 3(#18-20)+ 2 设计建议**
- 验证口径:每条均有 file:line 证据并经源码逐条核对(VERIFIED);agent 报告中不可复现的条目(nats/grpc 生命周期类 F7-F10)未晋升为缺陷,cluster 包 `-race` 单测与本席改动无交集且常规模式为绿。

## 八、commit 清单(本席最终汇总)

修复按**单点一个 commit** 落地(非原先计划的 7 个域级 commit),每项附对应回归测试,`todo.md` 逐项打勾;全部已推送。本席 finding → commit 映射:

| # | finding | commit |
|---|---|---|
| P0 #1 | 消息解码越界 | `b8a3e23` fix(message): reject overlong or unterminated message ID varint |
| P1 #2 | zip-bomb 无界解压 | `731295a` fix(compression): cap inflated payload size to prevent zip bombs |
| P1 #3/#5/#6 | 会话并发读 + Close 幂等 | `0e8014e` fix(session): make Close idempotent and guard concurrent data access |
| P1 #4 | uid 数据竞争 | `8846bbd` fix(session): make bind atomic and guard data races |
| P1 #7/#8 | acceptor 监听状态竞争 + TLS 标志 | `205fb8c` fix(acceptor): guard listener lifecycle with a mutex · `65ac329` fix(acceptor): guard nil listener on stop before listen |
| P1 #9 | 空服务器表 panic | `e500da7` fix(router): return error when no server is available · `86bbdd4` fix(router): guard nil result from custom route function |
| P1 #10 | nil reply reflect panic | `01f4395` fix(rpc): reject nil reply before reflect.TypeOf · `a2c87d5` fix(rpc): reject typed nil reply before unmarshal |
| P1 #11 | 心跳 interval<=0 panic | `3771328` fix(client): no-op heartbeat ticker on non-positive interval |
| P1 #12 | 拷贝互斥锁 | `c970864` fix(cluster): return pointer from parallel getter constructor |
| P2 #13/#14/#15 | fixture / unkeyed / gofmt | `b8a3e23`(fixture + golden) · `8b2c64b` style: gofmt and comment punctuation fixes |
| P2 #16 | agent `-race` 测试自身竞态 | `0e8014e`(随 session 修复一并落 `agent_test.go`) |

跟进加固(与本席 finding 同族,由 `todo.md` 审核项驱动):`bec9511` fix(message): support full 64-bit id and round-trip gzip · `6c3b8a1` fix(message): use uint64 for message id · `bb9fd1b` fix(session): harden frontend state and count · `7f556f3` / `1c39cdc` / `ca9af18` fix(session): IsFrontend 与可变视图并发加固 · `601486d` / `e4fce6d` fix(session): close lock 与 bind 回滚。

回归测试落点:`pkg/conn/message/message_test.go` · `pkg/session/session_test.go` · `pkg/router/router_test.go` · `pkg/rpc_test.go` · `pkg/client/client_test.go` · `pkg/acceptor/tcp_acceptor_test.go` · `pkg/util/compression/compression_test.go`(7 项,均已在质量门中通过)。

本报告为唯一新增未入库文件,随本 commit 提交。
