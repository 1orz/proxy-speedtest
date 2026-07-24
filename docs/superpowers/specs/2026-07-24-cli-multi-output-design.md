# CLI 多格式输出增强 — 设计文档

日期: 2026-07-24
分支: `feat/cli-multi-output`
目标版本: `v1.7.0`（对外发布前与用户确认）

## 背景

`proxy-speedtest` 已具备 CLI 能力：`-test <订阅>` 或裸分享链接参数都会走 `web.TestFromCMD` →
`web.TestContext` 直接测速；不传任何链接则启动 web 服务（默认 `:10888`）。当前 CLI 输出为
`json / text / pic / none` 四选一，且存在若干「硬编码文件名 + 重复写盘」的粗糙行为。

本次目标：在**不改动 web/WS/gRPC 路径**的前提下，补齐 CSV、支持一次产出多种格式、新增终端可读表格、
把测速进度打到 stderr、并修掉脏行为、刷新文档。

## 现状要点（已核实）

- 路由（`main.go`）：`-v` 版本 / `-grpc` gRPC / 有订阅或分享链接参数 → CLI 测速 /
  否则 web 服务。单条分享链接现在=直接测速（**不再**启动代理，README 相关章节已过时）。
- `internal/tester` 与 `internal/result` **未被任何代码引用**（死代码，另一套引擎）。本次不使用、不重接线。
- 数据模型 `web/render.Node`：`Id, Group, Remarks, Protocol, Ping(string), AvgSpeed, MaxSpeed,
  UploadSpeed, MaxUploadSpeed, Success, Traffic, Link`（速度单位为字节/秒 int64）。
- `web.testAll(ctx)` 同时负责「并发跑测速 + 收集」和「按 `OutputMode` 写盘/推图」。
  调用方：`web/server.go:171`（WS 路径 `go p.testAll`）与 `TestContext`（`server.go:434`）。
- 脏行为：
  - json 模式：`saveJSON` 硬写 `output.json`（忽略 `OutputFilePath`），`TestFromCMD` 又调 `outputJSON`
    写 `OutputFilePath`/stdout → **双写**，`output.json` 是多余产物。
  - `-output pic`：`renderPic` 在 `PIC_PATH` 下硬写 `out.png`（忽略 `OutputPicPath`）。
  - `-output text`：`saveText` 硬写 `output.txt`，且只导出测通节点的分享链接（非可读表格）。

## 架构决策：拆分 run 与 emit（方案 A）

把 `testAll` 中「并发跑 + 收集汇总（含 traffic/duration/success/links 计数）」抽成一个只跑、不写盘的方法，
返回一个汇总值；`testAll` 保留为「run + 现有 `OutputMode` 分支」，使 WS/HTTP 客户端行为**零改动**。
CLI 直接调用 run，然后交给全新的多格式 emitter。

```go
// TestSummary 承载一次测速的最终结果与汇总（供 CLI emitter 使用）。
type TestSummary struct {
    Nodes        render.Nodes // 按 Id 顺序（未排序）
    Traffic      int64
    Duration     string
    SuccessCount int
    LinksCount   int
}

// run 只跑测速并收集，不做任何写盘/推图；沿用现有 WriteMessage 逐节点进度消息。
func (p *ProfileTest) run(ctx context.Context) (render.Nodes, TestSummary, error)

// testAll 保持旧签名与行为：run + 现有 OutputMode 写盘/推图分支（WS/HTTP 用）。
func (p *ProfileTest) testAll(ctx context.Context) (render.Nodes, error)
```

- `run` 返回**未排序**（按 Id）的节点，保持 WS `PIC_NONE` 早返回时的既有顺序语义。
- 排序在各 emitter 内按 `SortMethod` 自行处理（CLI 默认 `rspeed`）。
- `TestContext` **保持不变**：仍调 `testAll` 返回 nodes（`OutputMode` 决定排序/写盘），
  以兼容 `examples/ping.go` 与外部调用。只有 CLI 专用入口（`TestFromCMD`，见下）改调 `run`。

## CLI 输出模型

### 参数

| Flag | 别名 | 默认 | 说明 |
|---|---|---|---|
| `-output` | `-o` | `json` | 逗号分隔的格式列表：`json,csv,text,table,pic,none` |
| `-output-file` | `-f` | | 数据类格式的落盘路径（见落盘规则） |
| `-output-pic` | | | PNG 落盘路径 |
| `-timeout -concurrency -download-url -download-size -threads -mode -config -log-level -p -bind` | | | 保持不变 |

别名实现：对同一变量 `flag.StringVar` 注册两次（`output`/`o`、`output-file`/`f`）。

### 6 种格式

- `json`：现有 `JSONOutput`（nodes+options+traffic+duration+计数），`MarshalIndent`。
- `csv`：`encoding/csv`，表头固定为：
  `id,group,remarks,protocol,ping_ms,avg_download_bytes_per_sec,max_download_bytes_per_sec,`
  `avg_upload_bytes_per_sec,max_upload_bytes_per_sec,traffic_bytes,success,link`
  - `ping_ms` 整数（失败=0）；上传列无数据=0；`success` 为 `true`/`false`；速度单位字节/秒（与 JSON 一致）。
  - **速度不额外加人类可读列**（保持机器友好；人看用 `table`）。
- `text`：测通节点（`Success` 且 `Ping!="0"` 或有速度）的分享链接，每行一条（保留旧语义，可当订阅导入）。
- `table`：终端可读对齐表（`text/tabwriter`）。列：`#  Remarks  Proto  Ping  Down  Up`，
  底部汇总行：`Total N · OK M · Traffic X · <duration>`。失败节点 Ping/Down 显示 `-`；
  Remarks 截断到约 28 runes；速度用 `download.ByteCountIECTrim`。
- `pic`：现有 `render.Table` PNG。
- `none`：不产出；与其它格式并列时视为 no-op（若列表仅 `none` 则无任何输出）。

### 落盘规则（可预测）

数据类 = {json, csv, text, table}；图片类 = {pic}。

- 数据类：
  - 未给 `-output-file` 且仅 1 种 → **stdout**。
  - 未给 `-output-file` 且 ≥2 种 → 各写 `speedtest-<时间戳>.<ext>`（`slog.Info` 打印路径），stdout 无输出。
  - 给了 `-output-file` → 第 1 种数据类用它（自动替换为该格式扩展名），其余自动命名；stdout 无输出。
- 图片类：**永远落盘**。路径优先 `-output-pic`；否则若 pic 是唯一输出且给了 `-output-file` 用它；
  否则 `speedtest-<时间戳>.png`。落盘后 `slog.Info` 打印路径。
- 扩展名映射：json→`.json`、csv→`.csv`、text→`.txt`、table→`.txt`、pic→`.png`。
- 时间戳格式：`speedtest-YYYYMMDD-HHMMSS.<ext>`（本地时区，秒级）。

### 解析与校验（OutputPlan）

- 拆分 `-output` 逗号 → trim/lower/去重、保序。空 → `["json"]`。
- 每项须 ∈ 合法集合，否则 `fatal`（明确列出合法值）。
- `none` 若与其它项并存则丢弃 `none`；若仅 `none` 则本次不产出数据/图片（测速仍执行，可配合非零退出码/日志）。
- 产出一个内部 `OutputPlan`：`[]OutputTarget{ Format, Dest(stdout|filePath) }`（纯函数，便于单测）。

## stderr 进度

新增 `CMDProgressWriter`（实现 `MessageWriter`）：解析既有消息（`gotservers/gotserver` 注册每节点
Remarks/Protocol 与总数；`gotping` 存 ping；`gotspeed`/`gotupload` 存下/上行；`endone` 触发打印该节点一行）。

- 行样式：`[done/total] ✓ <Remarks>  ping <n>ms  ↓ <down>  ↑ <up>`；失败：`[done/total] ✗ <Remarks>  failed`。
- 全部打到 **stderr**；stdout 只留纯净数据，便于管道（`... | jq`、`... > r.csv`）。
- 结尾汇总行由 emitter 打到 stderr。
- `-log-level silent` → 用 `EmptyMessageWriter`（无进度）。

## CLI 主流程（`TestFromCMD` 重写）

1. 由 config + cmdOpts 组装 `ProfileTestOptions`（沿用现有覆盖逻辑；不再用单一 `OutputMode` int 决定 CLI 输出）。
2. 解析 `OutputPlan`（校验失败 fatal）。
3. `Writer` = 非 silent 时 `CMDProgressWriter`，否则 `EmptyMessageWriter`。
4. `nodes, summary, err := p.run(ctx)`。
5. `emitOutputs(summary, plan, opts)`：按 plan 逐目标渲染；任一写盘失败 → 返回错误（非零退出）。

## 破坏性变更（均为改善，README 说明）

- 移除 json 模式多余的 `output.json` 双写。
- `-output pic` 不再硬写 `out.png`（改 `-output-pic`/时间戳名）。
- `-output text` 不再硬写 `output.txt`（改 stdout/`-output-file`）。
- 未知格式/无节点/写盘失败 → 非零退出。

## 文档（README）

- 重写 CLI 用法：订阅或单分享链接 → CLI；无参数 → web `:10888`。
- 新增格式表（json/csv/table/text/pic/none）、CSV 列说明、落盘规则、别名、管道示例。
- **删除**过时的「Run as a HTTP/SOCKS5 proxy（`./proxy-speedtest vmess://...`）」章节（现单链接=直接测速）。

## 测试

新逻辑集中在纯函数，重点单测（`web` 包）：

- OutputPlan 解析：逗号/去重/保序、空→json、未知→错误、none 规则、`-output-file`/多格式的落盘目标解析。
- CSV 编码：表头正确；含逗号/引号的 Remarks 被正确转义；上传/失败列取值正确。
- table 渲染：包含预期列头与汇总子串；失败节点显示 `-`。
- 时间戳文件名 helper：注入时间可确定化断言。
- 保持 `go test ./...` 全绿（CI 已跑）。真实联网测速不进 CI；本地用 `verify` 跑一次真实小样本验证端到端。

## 交付流程

1. 功能分支 `feat/cli-multi-output` 上 TDD 实现，`go build` + `go test ./...` 绿。
2. 本地 `verify` 端到端跑一次。
3. 合并到 `main`。
4. 与用户确认版本号（拟 `v1.7.0`）后打 tag 推送，触发 CI 多平台二进制 + Docker 发布。

## 非目标（YAGNI）

- 不重接 `internal/tester`/`internal/result`。
- 不新增上传/代理相关 CLI 能力。
- CSV 不加人类可读速度列。
- 不改 web/WS/gRPC 行为。
