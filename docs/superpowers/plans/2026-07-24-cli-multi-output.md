# CLI 多格式输出增强 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 CLI 增加 CSV、终端表格、一次多格式产出、stderr 进度，并修掉硬编码/双写脏行为，不改动 web/WS/gRPC 行为。

**Architecture:** 把 `web.testAll` 拆成「只跑不写盘的 `run` + 保留旧行为的 `testAll`」；CLI 走 `run` + 全新的纯函数渲染器与 `emitCMD` 路由；输出计划由纯函数 `ParseOutputPlan` 决定 stdout/文件。

**Tech Stack:** Go 1.26；标准库 `encoding/csv`、`text/tabwriter`；复用 `web/render`、`download` 包。

## Global Constraints

- Go module `github.com/1orz/proxy-speedtest`，包 `web`（新代码放 `web/` 下）。
- 合法输出格式集合（逐字）：`json, csv, text, table, pic, none`。
- 时间戳文件名格式：`speedtest-<YYYYMMDD-HHMMSS>.<ext>`（`now.Format("20060102-150405")`）。
- 扩展名映射：json→`.json`，csv→`.csv`，text→`.txt`，table→`.txt`，pic→`.png`。
- CSV 表头（逐字，固定顺序）：
  `id,group,remarks,protocol,ping_ms,avg_download_bytes_per_sec,max_download_bytes_per_sec,avg_upload_bytes_per_sec,max_upload_bytes_per_sec,traffic_bytes,success,link`
- stdout 只输出数据类结果；进度/日志一律 stderr。
- `-log-level silent` 时不打印进度。
- 不改 `web/server.go` 的 WS handler、`TestContext`、`examples/ping.go` 的行为。
- 每个任务结束跑 `go build ./... && go test ./...` 必须绿。

---

### Task 1: OutputPlan 解析（纯函数）

**Files:**
- Create: `web/cmdoutput.go`
- Test: `web/cmdoutput_test.go`

**Interfaces:**
- Produces:
  - `type OutputTarget struct { Format string; Path string }` （Path=="" 表示 stdout；仅数据类可 stdout）
  - `func ParseOutputPlan(spec, outputFile, outputPic string, now time.Time) ([]OutputTarget, error)`
  - `func timestampName(now time.Time, ext string) string`
  - `func extFor(format string) string`

- [ ] **Step 1: 写失败测试** `web/cmdoutput_test.go`

```go
package web

import (
	"testing"
	"time"
)

func fixedNow() time.Time { return time.Date(2026, 7, 24, 15, 30, 5, 0, time.UTC) }

func TestParseOutputPlan(t *testing.T) {
	ts := "speedtest-20260724-153005"
	cases := []struct {
		name       string
		spec, file, pic string
		want       []OutputTarget
		wantErr    bool
	}{
		{"default empty -> json stdout", "", "", "", []OutputTarget{{"json", ""}}, false},
		{"single csv stdout", "csv", "", "", []OutputTarget{{"csv", ""}}, false},
		{"single csv to file", "csv", "r.csv", "", []OutputTarget{{"csv", "r.csv"}}, false},
		{"dedupe + order", "json,json,csv", "", "", []OutputTarget{{"json", ts + ".json"}, {"csv", ts + ".csv"}}, false},
		{"two data no file -> auto files", "json,csv", "", "", []OutputTarget{{"json", ts + ".json"}, {"csv", ts + ".csv"}}, false},
		{"first data uses file verbatim, extra auto", "json,csv", "out.json", "", []OutputTarget{{"json", "out.json"}, {"csv", ts + ".csv"}}, false},
		{"json + pic stdout + default png", "json,pic", "", "", []OutputTarget{{"json", ""}, {"pic", ts + ".png"}}, false},
		{"pic only default png", "pic", "", "", []OutputTarget{{"pic", ts + ".png"}}, false},
		{"pic only with output-pic", "pic", "", "p.png", []OutputTarget{{"pic", "p.png"}}, false},
		{"pic only, no pic path, uses output-file", "pic", "shot.png", "", []OutputTarget{{"pic", "shot.png"}}, false},
		{"none only -> empty", "none", "", "", []OutputTarget{}, false},
		{"none combined dropped", "json,none", "", "", []OutputTarget{{"json", ""}}, false},
		{"unknown -> error", "json,foo", "", "", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseOutputPlan(c.spec, c.file, c.pic, fixedNow())
			if c.wantErr {
				if err == nil { t.Fatalf("expected error, got nil") }
				return
			}
			if err != nil { t.Fatalf("unexpected error: %v", err) }
			if len(got) != len(c.want) { t.Fatalf("len=%d want=%d (%v)", len(got), len(c.want), got) }
			for i := range got {
				if got[i] != c.want[i] { t.Fatalf("target[%d]=%+v want %+v", i, got[i], c.want[i]) }
			}
		})
	}
}
```

- [ ] **Step 2: 跑测试确认失败** `go test ./web/ -run TestParseOutputPlan -v` → FAIL（未定义）

- [ ] **Step 3: 实现** `web/cmdoutput.go`

```go
package web

import (
	"fmt"
	"strings"
	"time"
)

// OutputTarget 是一个已解析的输出目标。Path == "" 表示写 stdout(仅数据类)。
type OutputTarget struct {
	Format string // json|csv|text|table|pic
	Path   string
}

var validOutputFormats = map[string]bool{
	"json": true, "csv": true, "text": true, "table": true, "pic": true, "none": true,
}

func extFor(format string) string {
	switch format {
	case "json":
		return ".json"
	case "csv":
		return ".csv"
	case "text", "table":
		return ".txt"
	case "pic":
		return ".png"
	}
	return ".out"
}

func timestampName(now time.Time, ext string) string {
	return "speedtest-" + now.Format("20060102-150405") + ext
}

// ParseOutputPlan 把 -output 逗号列表 + -output-file/-output-pic 解析为一组输出目标。
// 规则见设计文档:数据类未给文件且仅一种 -> stdout;多种或与文件配合 -> 首个数据类用文件、其余时间戳自动命名;
// 图片类永远落盘。
func ParseOutputPlan(spec, outputFile, outputPic string, now time.Time) ([]OutputTarget, error) {
	seen := map[string]bool{}
	var formats []string
	for _, part := range strings.Split(spec, ",") {
		f := strings.ToLower(strings.TrimSpace(part))
		if f == "" {
			continue
		}
		if !validOutputFormats[f] {
			return nil, fmt.Errorf("unknown output format %q (valid: json, csv, text, table, pic, none)", f)
		}
		if !seen[f] {
			seen[f] = true
			formats = append(formats, f)
		}
	}
	if len(formats) == 0 {
		formats = []string{"json"}
	}
	if len(formats) == 1 && formats[0] == "none" {
		return []OutputTarget{}, nil
	}
	if len(formats) > 1 {
		kept := formats[:0:0]
		for _, f := range formats {
			if f != "none" {
				kept = append(kept, f)
			}
		}
		formats = kept
	}

	dataCount := 0
	for _, f := range formats {
		if f != "pic" {
			dataCount++
		}
	}

	var targets []OutputTarget
	dataFileUsed := false
	for _, f := range formats {
		if f == "pic" {
			path := outputPic
			if path == "" {
				if dataCount == 0 && outputFile != "" {
					path = outputFile
				} else {
					path = timestampName(now, ".png")
				}
			}
			targets = append(targets, OutputTarget{Format: "pic", Path: path})
			continue
		}
		switch {
		case outputFile != "" && !dataFileUsed:
			dataFileUsed = true
			targets = append(targets, OutputTarget{Format: f, Path: outputFile})
		case outputFile == "" && dataCount == 1:
			targets = append(targets, OutputTarget{Format: f, Path: ""})
		default:
			targets = append(targets, OutputTarget{Format: f, Path: timestampName(now, extFor(f))})
		}
	}
	return targets, nil
}
```

- [ ] **Step 4: 跑测试确认通过** `go test ./web/ -run TestParseOutputPlan -v` → PASS

- [ ] **Step 5: 提交**

```bash
git add web/cmdoutput.go web/cmdoutput_test.go
git commit -m "feat(cli): add ParseOutputPlan for multi-format output routing"
```

---

### Task 2: TestSummary + CSV / JSON / text 渲染器（纯函数）

**Files:**
- Modify: `web/cmdoutput.go`
- Test: `web/cmdoutput_test.go`

**Interfaces:**
- Produces:
  - `type TestSummary struct { Nodes render.Nodes; Traffic int64; Duration string; SuccessCount int; LinksCount int }`
  - `func renderCSVBytes(summary TestSummary) ([]byte, error)`
  - `func renderTextBytes(summary TestSummary) []byte`
  - `func renderJSONBytes(summary TestSummary, opts *ProfileTestOptions) ([]byte, error)`
- Consumes: `render.Node`（字段 Id/Group/Remarks/Protocol/Ping(string)/AvgSpeed/MaxSpeed/UploadSpeed/MaxUploadSpeed/Traffic/Success/Link）；`JSONOutput`（web/profile.go 现有）。

- [ ] **Step 1: 写失败测试**（追加到 `web/cmdoutput_test.go`）

```go
import (
	"encoding/csv"
	"strings"

	"github.com/1orz/proxy-speedtest/web/render"
)

func sampleSummary() TestSummary {
	return TestSummary{
		Nodes: render.Nodes{
			{Id: 0, Group: "g", Remarks: "HK, Premium", Protocol: "vmess/ws", Ping: "42",
				AvgSpeed: 1000, MaxSpeed: 2000, UploadSpeed: 300, MaxUploadSpeed: 500,
				Traffic: 99, Success: true, Link: "vmess://a"},
			{Id: 1, Group: "g", Remarks: "Dead", Protocol: "ss", Ping: "0",
				AvgSpeed: 0, MaxSpeed: 0, Success: false, Link: "ss://b"},
		},
		Traffic: 99, Duration: "0m 3s", SuccessCount: 1, LinksCount: 2,
	}
}

func TestRenderCSVBytes(t *testing.T) {
	b, err := renderCSVBytes(sampleSummary())
	if err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(strings.NewReader(string(b)))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv not parseable: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows=%d want 3", len(rows))
	}
	wantHeader := []string{"id", "group", "remarks", "protocol", "ping_ms",
		"avg_download_bytes_per_sec", "max_download_bytes_per_sec",
		"avg_upload_bytes_per_sec", "max_upload_bytes_per_sec",
		"traffic_bytes", "success", "link"}
	for i, h := range wantHeader {
		if rows[0][i] != h {
			t.Fatalf("header[%d]=%q want %q", i, rows[0][i], h)
		}
	}
	// remarks with comma preserved as single field
	if rows[1][2] != "HK, Premium" {
		t.Fatalf("remarks=%q want %q", rows[1][2], "HK, Premium")
	}
	if rows[1][10] != "true" || rows[2][10] != "false" {
		t.Fatalf("success col wrong: %q / %q", rows[1][10], rows[2][10])
	}
	if rows[1][4] != "42" || rows[2][4] != "0" {
		t.Fatalf("ping col wrong: %q / %q", rows[1][4], rows[2][4])
	}
	if rows[1][5] != "1000" || rows[1][7] != "300" {
		t.Fatalf("speed cols wrong: %q / %q", rows[1][5], rows[1][7])
	}
}

func TestRenderTextBytes(t *testing.T) {
	got := string(renderTextBytes(sampleSummary()))
	if got != "vmess://a" {
		t.Fatalf("text=%q want only working link", got)
	}
}

func TestRenderJSONBytes(t *testing.T) {
	opts := &ProfileTestOptions{GroupName: "Default"}
	b, err := renderJSONBytes(sampleSummary(), opts)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "\"successCount\": 1") || !strings.Contains(s, "\"linksCount\": 2") {
		t.Fatalf("json missing summary counts: %s", s)
	}
}
```

- [ ] **Step 2: 跑测试确认失败** `go test ./web/ -run 'TestRender' -v` → FAIL

- [ ] **Step 3: 实现**（追加到 `web/cmdoutput.go`；补充 import）

```go
// 追加 import: "bytes" "encoding/csv" "encoding/json" "strconv"
//   "github.com/1orz/proxy-speedtest/web/render"

// TestSummary 承载一次测速的最终结果与汇总(供 CLI emitter)。
type TestSummary struct {
	Nodes        render.Nodes
	Traffic      int64
	Duration     string
	SuccessCount int
	LinksCount   int
}

func renderCSVBytes(summary TestSummary) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	header := []string{"id", "group", "remarks", "protocol", "ping_ms",
		"avg_download_bytes_per_sec", "max_download_bytes_per_sec",
		"avg_upload_bytes_per_sec", "max_upload_bytes_per_sec",
		"traffic_bytes", "success", "link"}
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, n := range summary.Nodes {
		ping := n.Ping
		if ping == "" {
			ping = "0"
		}
		row := []string{
			strconv.Itoa(n.Id), n.Group, n.Remarks, n.Protocol, ping,
			strconv.FormatInt(n.AvgSpeed, 10), strconv.FormatInt(n.MaxSpeed, 10),
			strconv.FormatInt(n.UploadSpeed, 10), strconv.FormatInt(n.MaxUploadSpeed, 10),
			strconv.FormatInt(n.Traffic, 10), strconv.FormatBool(n.Success), n.Link,
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func renderTextBytes(summary TestSummary) []byte {
	var links []string
	for _, n := range summary.Nodes {
		if n.Ping != "0" || n.AvgSpeed > 0 || n.MaxSpeed > 0 {
			links = append(links, n.Link)
		}
	}
	return []byte(strings.Join(links, "\n"))
}

func renderJSONBytes(summary TestSummary, opts *ProfileTestOptions) ([]byte, error) {
	out := JSONOutput{
		Nodes:        summary.Nodes,
		Options:      *opts,
		Traffic:      summary.Traffic,
		Duration:     summary.Duration,
		SuccessCount: summary.SuccessCount,
		LinksCount:   summary.LinksCount,
	}
	return json.MarshalIndent(&out, "", "  ")
}
```

- [ ] **Step 4: 跑测试确认通过** `go test ./web/ -run 'TestRender' -v` → PASS

- [ ] **Step 5: 提交**

```bash
git add web/cmdoutput.go web/cmdoutput_test.go
git commit -m "feat(cli): add TestSummary + csv/text/json renderers"
```

---

### Task 3: 终端表格渲染器（纯函数）

**Files:**
- Modify: `web/cmdoutput.go`
- Test: `web/cmdoutput_test.go`

**Interfaces:**
- Produces: `func renderTableString(summary TestSummary) string`；`func truncateRunes(s string, n int) string`
- Consumes: `download.ByteCountIECTrim(int64) string`

- [ ] **Step 1: 写失败测试**（追加）

```go
func TestRenderTableString(t *testing.T) {
	s := renderTableString(sampleSummary())
	for _, sub := range []string{"REMARKS", "PROTO", "PING", "DOWN", "UP", "Total 2", "OK 1"} {
		if !strings.Contains(s, sub) {
			t.Fatalf("table missing %q:\n%s", sub, s)
		}
	}
	if !strings.Contains(s, "42ms") {
		t.Fatalf("table missing working ping:\n%s", s)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("abcdef", 4); got != "abc…" {
		t.Fatalf("truncate=%q", got)
	}
	if got := truncateRunes("ab", 4); got != "ab" {
		t.Fatalf("truncate=%q", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败** `go test ./web/ -run 'Table|Truncate' -v` → FAIL

- [ ] **Step 3: 实现**（追加；补 import `"fmt"` `"text/tabwriter"` `"github.com/1orz/proxy-speedtest/download"`）

```go
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return string(r[:n-1]) + "…"
}

func renderTableString(summary TestSummary) string {
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tREMARKS\tPROTO\tPING\tDOWN\tUP")
	for i, n := range summary.Nodes {
		ping, down, up := "-", "-", "-"
		if n.Success {
			if n.Ping != "0" && n.Ping != "" {
				ping = n.Ping + "ms"
			}
			if n.AvgSpeed > 0 {
				down = download.ByteCountIECTrim(n.AvgSpeed)
			}
			if n.UploadSpeed > 0 {
				up = download.ByteCountIECTrim(n.UploadSpeed)
			}
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
			i+1, truncateRunes(n.Remarks, 28), n.Protocol, ping, down, up)
	}
	tw.Flush()
	b.WriteString(fmt.Sprintf("— Total %d · OK %d · Traffic %s · %s\n",
		summary.LinksCount, summary.SuccessCount,
		download.ByteCountIECTrim(summary.Traffic), summary.Duration))
	return b.String()
}
```

- [ ] **Step 4: 跑测试确认通过** `go test ./web/ -run 'Table|Truncate' -v` → PASS

- [ ] **Step 5: 提交**

```bash
git add web/cmdoutput.go web/cmdoutput_test.go
git commit -m "feat(cli): add terminal table renderer"
```

---

### Task 4: stderr 进度写入器

**Files:**
- Create: `web/cmdprogress.go`
- Test: `web/cmdprogress_test.go`

**Interfaces:**
- Produces:
  - `type CMDProgressWriter struct {...}` 实现 `MessageWriter`（`WriteMessage(int, []byte) error`）
  - `func NewCMDProgressWriter(out io.Writer) *CMDProgressWriter`
  - `func formatProgressLine(done, total int, np nodeProgress) string`
  - `type nodeProgress struct { remarks, protocol, down, up string; ping int64; pingSet bool }`
- Consumes: `Message`（web/msg.go：字段 ID/Info/Servers[]/Ping/Speed/UploadSpeed/Remarks/Protocol）

- [ ] **Step 1: 写失败测试** `web/cmdprogress_test.go`

```go
package web

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatProgressLine(t *testing.T) {
	ok := formatProgressLine(1, 3, nodeProgress{remarks: "HK", ping: 42, pingSet: true, down: "12.3 MiB", up: "3.1 MiB"})
	if !strings.Contains(ok, "[1/3]") || !strings.Contains(ok, "✓") ||
		!strings.Contains(ok, "ping 42ms") || !strings.Contains(ok, "12.3 MiB") {
		t.Fatalf("ok line wrong: %q", ok)
	}
	fail := formatProgressLine(2, 3, nodeProgress{remarks: "Dead", ping: 0, pingSet: true, down: "N/A"})
	if !strings.Contains(fail, "✗") || !strings.Contains(fail, "failed") {
		t.Fatalf("fail line wrong: %q", fail)
	}
	speedOnly := formatProgressLine(3, 3, nodeProgress{remarks: "S", down: "5.0 MiB"})
	if !strings.Contains(speedOnly, "✓") {
		t.Fatalf("speed-only should be ok: %q", speedOnly)
	}
}

func TestCMDProgressWriterFlow(t *testing.T) {
	var buf bytes.Buffer
	w := NewCMDProgressWriter(&buf)
	_ = w.WriteMessage(1, []byte(`{"id":0,"info":"gotservers","servers":[{"id":0,"info":"gotserver","remarks":"HK","protocol":"vmess"},{"id":1,"info":"gotserver","remarks":"JP","protocol":"ss"}]}`))
	_ = w.WriteMessage(1, []byte(`{"id":0,"info":"gotping","ping":42}`))
	_ = w.WriteMessage(1, []byte(`{"id":0,"info":"gotspeed","speed":"12.3 MiB"}`))
	_ = w.WriteMessage(1, []byte(`{"id":0,"info":"endone"}`))
	_ = w.WriteMessage(1, []byte(`{"id":1,"info":"gotping","ping":0}`))
	_ = w.WriteMessage(1, []byte(`{"id":1,"info":"endone"}`))
	out := buf.String()
	if !strings.Contains(out, "[1/2]") || !strings.Contains(out, "HK") || !strings.Contains(out, "ping 42ms") {
		t.Fatalf("progress out missing node 0: %q", out)
	}
	if !strings.Contains(out, "[2/2]") || !strings.Contains(out, "JP") {
		t.Fatalf("progress out missing node 1: %q", out)
	}
}
```

- [ ] **Step 2: 跑测试确认失败** `go test ./web/ -run 'Progress' -v` → FAIL

- [ ] **Step 3: 实现** `web/cmdprogress.go`

```go
package web

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

type nodeProgress struct {
	remarks  string
	protocol string
	down     string
	up       string
	ping     int64
	pingSet  bool
}

func formatProgressLine(done, total int, np nodeProgress) string {
	ok := (np.pingSet && np.ping > 0) || (np.down != "" && np.down != "N/A")
	status := "✗" // ✗
	metrics := "failed"
	if ok {
		status = "✓" // ✓
		var parts []string
		if np.pingSet && np.ping > 0 {
			parts = append(parts, fmt.Sprintf("ping %dms", np.ping))
		}
		if np.down != "" && np.down != "N/A" {
			parts = append(parts, "↓ "+np.down)
		}
		if np.up != "" && np.up != "N/A" {
			parts = append(parts, "↑ "+np.up)
		}
		if len(parts) == 0 {
			parts = append(parts, "ok")
		}
		metrics = strings.Join(parts, "  ")
	}
	return fmt.Sprintf("[%d/%d] %s %s  %s", done, total, status, np.remarks, metrics)
}

// CMDProgressWriter 消费测速过程消息,逐节点把进度打到 out(通常 os.Stderr)。
type CMDProgressWriter struct {
	mu    sync.Mutex
	nodes map[int]*nodeProgress
	total int
	done  int
	out   io.Writer
}

func NewCMDProgressWriter(out io.Writer) *CMDProgressWriter {
	return &CMDProgressWriter{nodes: map[int]*nodeProgress{}, out: out}
}

func (w *CMDProgressWriter) WriteMessage(_ int, data []byte) error {
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return nil // 非结构化消息忽略
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	switch m.Info {
	case "gotservers":
		for _, s := range m.Servers {
			if _, ok := w.nodes[s.ID]; !ok {
				w.total++
			}
			w.nodes[s.ID] = &nodeProgress{remarks: s.Remarks, protocol: s.Protocol}
		}
	case "gotping":
		if np := w.nodes[m.ID]; np != nil {
			np.ping = m.Ping
			np.pingSet = true
		}
	case "gotspeed":
		if np := w.nodes[m.ID]; np != nil {
			np.down = m.Speed
		}
	case "gotupload":
		if np := w.nodes[m.ID]; np != nil {
			np.up = m.UploadSpeed
		}
	case "endone":
		w.done++
		np := w.nodes[m.ID]
		if np == nil {
			np = &nodeProgress{remarks: fmt.Sprintf("Profile %d", m.ID)}
		}
		fmt.Fprintln(w.out, formatProgressLine(w.done, w.total, *np))
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过** `go test ./web/ -run 'Progress' -v` → PASS

- [ ] **Step 5: 提交**

```bash
git add web/cmdprogress.go web/cmdprogress_test.go
git commit -m "feat(cli): add stderr progress writer"
```

---

### Task 5: 拆分 testAll → run（保持 WS 行为不变）

**Files:**
- Modify: `web/profile.go`（`testAll` 约 528-626 行）

**Interfaces:**
- Produces: `func (p *ProfileTest) run(ctx context.Context) (render.Nodes, TestSummary, error)`
- 保持不变: `func (p *ProfileTest) testAll(ctx context.Context) (render.Nodes, error)`（现在 = `run` + 原 OutputMode 分支）

- [ ] **Step 1: 重构实现** — 把 `testAll` 拆成两段。`run` 负责跑测速与收集(含 started/gotservers/eof 消息、IP 抓取、duration/traffic/successCount 计算),返回未排序 nodes + summary,**不排序、不写盘**。`testAll` 调 `run` 后照旧按 `OutputMode` 排序并写盘/推图。

`web/profile.go` 用下面内容替换现有 `testAll`（528-626 行整段）:

```go
// run 只跑测速并收集汇总,不排序、不写盘/推图。WS 与 CLI 共用。
func (p *ProfileTest) run(ctx context.Context) (render.Nodes, TestSummary, error) {
	linksCount := len(p.Links)
	if linksCount < 1 {
		p.WriteString(SPEEDTEST_ERROR_NONODES)
		return nil, TestSummary{}, fmt.Errorf("no profile found")
	}
	start := time.Now()
	p.WriteMessage(getMsgByte(-1, "started"))
	if p.shouldFetchIP() {
		p.startIPFetch(ctx)
	}
	step := 9
	if linksCount > 200 {
		step = linksCount / 20
		if step > 50 {
			step = 50
		}
	}
	for i := 0; i < linksCount; {
		end := i + step
		if end > linksCount {
			end = linksCount
		}
		links := p.Links[i:end]
		groups := make([]string, len(links))
		for j := range links {
			groups[j] = p.groupFor(i + j)
		}
		p.WriteMessage(gotserversMsg(i, links, groups))
		i += step
	}
	guard := make(chan int, p.Options.Concurrency)
	nodeChan := make(chan render.Node, linksCount)
	nodes := make(render.Nodes, linksCount)
	for i := range p.Links {
		p.wg.Add(1)
		id := i
		link := ""
		if len(p.Options.TestIDs) > 0 && len(p.Options.Links) > 0 {
			id = p.Options.TestIDs[i]
			link = p.Options.Links[i]
		}
		select {
		case guard <- i:
			go func(id int, link string, c <-chan int, nodeChan chan<- render.Node) {
				p.testOne(ctx, id, link, nodeChan, nil)
				_ = p.WriteMessage(getMsgByte(id, "endone"))
				<-c
			}(id, link, guard, nodeChan)
		case <-ctx.Done():
			return nil, TestSummary{}, ctx.Err()
		}
	}
	p.wg.Wait()
	p.WriteMessage(getMsgByte(-1, "eof"))
	duration := FormatDuration(time.Since(start))
	successCount := 0
	var traffic int64 = 0
	for i := 0; i < linksCount; i++ {
		node := <-nodeChan
		node.Link = p.Links[node.Id]
		nodes[node.Id] = node
		traffic += node.Traffic
		if node.Success {
			successCount += 1
		}
	}
	close(nodeChan)
	summary := TestSummary{
		Nodes:        nodes,
		Traffic:      traffic,
		Duration:     duration,
		SuccessCount: successCount,
		LinksCount:   linksCount,
	}
	return nodes, summary, nil
}

// testAll 保持旧签名与副作用:run + 按 OutputMode 排序/写盘/推图(WS 与 HTTP 客户端用)。
func (p *ProfileTest) testAll(ctx context.Context) (render.Nodes, error) {
	nodes, summary, err := p.run(ctx)
	if err != nil {
		return nil, err
	}
	if p.Options.OutputMode == PIC_NONE {
		return nodes, nil
	}
	nodes.Sort(p.Options.SortMethod)
	traffic, duration := summary.Traffic, summary.Duration
	successCount, linksCount := summary.SuccessCount, summary.LinksCount
	if p.Options.OutputMode == JSON_OUTPUT {
		p.saveJSON(nodes, traffic, duration, successCount, linksCount)
	} else if p.Options.OutputMode == TEXT_OUTPUT {
		p.saveText(nodes)
	} else {
		p.renderPic(nodes, traffic, duration, successCount, linksCount)
	}
	if p.Options.OutputPicPath != "" && p.Options.OutputMode != PIC_PATH && p.Options.OutputMode != PIC_BASE64 {
		p.savePic(nodes, traffic, duration, successCount, linksCount)
	}
	return nodes, nil
}
```

- [ ] **Step 2: 构建 + 全量测试**

Run: `go build ./... && go test ./...`
Expected: PASS（WS 行为不变;`TestContext` 仍调 `testAll`）

- [ ] **Step 3: 提交**

```bash
git add web/profile.go
git commit -m "refactor(web): split testAll into run(collect) + testAll(emit)"
```

---

### Task 6: emitCMD 路由 + CLI 图片落盘

**Files:**
- Modify: `web/cmdoutput.go`

**Interfaces:**
- Produces:
  - `func (p *ProfileTest) emitCMD(summary TestSummary, targets []OutputTarget) error`
  - `func (p *ProfileTest) savePicPath(summary TestSummary, path string) error`
- Consumes: 前述渲染器；`render.NewTableOptions/NewTableWithOption`、`FontBytes`、`p.ipInfoLines()`、`table.FormatTraffic/Draw`（照搬现有 `savePic`）

- [ ] **Step 1: 实现**（追加到 `web/cmdoutput.go`；补 import `"log/slog"` `"os"`）

```go
// emitCMD 按 targets 渲染并输出。数据类节点先按 SortMethod 排序(副本,不动 summary)。
func (p *ProfileTest) emitCMD(summary TestSummary, targets []OutputTarget) error {
	sorted := make(render.Nodes, len(summary.Nodes))
	copy(sorted, summary.Nodes)
	sorted.Sort(p.Options.SortMethod)
	s := summary
	s.Nodes = sorted

	for _, t := range targets {
		if t.Format == "pic" {
			if err := p.savePicPath(s, t.Path); err != nil {
				return fmt.Errorf("write pic %s: %w", t.Path, err)
			}
			slog.Info("pic result saved", "path", t.Path)
			continue
		}
		var data []byte
		var err error
		switch t.Format {
		case "json":
			data, err = renderJSONBytes(s, p.Options)
		case "csv":
			data, err = renderCSVBytes(s)
		case "text":
			data = renderTextBytes(s)
		case "table":
			data = []byte(renderTableString(s))
		default:
			continue
		}
		if err != nil {
			return err
		}
		if t.Path == "" {
			if _, err := os.Stdout.Write(data); err != nil {
				return err
			}
			if len(data) == 0 || data[len(data)-1] != '\n' {
				fmt.Fprintln(os.Stdout)
			}
		} else {
			if err := os.WriteFile(t.Path, data, 0644); err != nil {
				return fmt.Errorf("write %s: %w", t.Path, err)
			}
			slog.Info("result saved", "format", t.Format, "path", t.Path)
		}
	}
	return nil
}

// savePicPath 把结果渲染为 PNG 到指定路径(照搬 savePic,但路径显式传入)。
func (p *ProfileTest) savePicPath(summary TestSummary, path string) error {
	fontPath := "WenQuanYiMicroHei-01.ttf"
	options := render.NewTableOptions(40, 30, 0.5, 0.5, p.Options.FontSize, 0.5, fontPath,
		p.Options.Language, p.Options.Theme, "Asia/Shanghai", FontBytes)
	options.SetAppearance(p.Options.Appearance)
	v4Line, v6Line := p.ipInfoLines()
	options.SetIPInfo(v4Line, v6Line)
	table, err := render.NewTableWithOption(summary.Nodes, &options)
	if err != nil {
		return err
	}
	msg := table.FormatTraffic(download.ByteCountIECTrim(summary.Traffic), summary.Duration,
		fmt.Sprintf("%d/%d", summary.SuccessCount, summary.LinksCount))
	table.Draw(path, msg)
	return nil
}
```

- [ ] **Step 2: 构建 + 全量测试**

Run: `go build ./... && go test ./...`
Expected: PASS

- [ ] **Step 3: 提交**

```bash
git add web/cmdoutput.go
git commit -m "feat(cli): add emitCMD router + explicit-path pic saver"
```

---

### Task 7: 重写 TestFromCMD 走新流程 + CMDOptions.Silent

**Files:**
- Modify: `web/server.go`（`TestFromCMD` 291-385 行；`outputJSON` 387-419 行删除）
- Modify: `web/profile.go`（`CMDOptions` 结构 312-322 行，加 `Silent bool`）

**Interfaces:**
- Consumes: `ParseOutputPlan`、`(*ProfileTest).run`、`(*ProfileTest).emitCMD`、`NewCMDProgressWriter`、`EmptyMessageWriter`、`ParseLinks`

- [ ] **Step 1: CMDOptions 加字段** — 在 `web/profile.go` 的 `CMDOptions` 末尾加：

```go
	Mode          string // pingonly, speedonly, all
	Silent        bool   // -log-level silent: 不打印 stderr 进度
```

- [ ] **Step 2: 重写 TestFromCMD** — 用下面替换 `web/server.go` 的 `TestFromCMD` 全函数（并删除其后的 `outputJSON`）：

```go
func TestFromCMD(subscription string, configPath *string, cmdOpts *CMDOptions) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	options := ProfileTestOptions{
		GroupName:     "Default",
		SpeedTestMode: "all",
		SortMethod:    "rspeed",
		Concurrency:   2,
		TestMode:      2,
		Subscription:  subscription,
		Language:      "en",
		FontSize:      24,
		Theme:         "rainbow",
		Appearance:    "light",
		Timeout:       15 * time.Second,
		OutputMode:    PIC_NONE, // CLI 不用 OutputMode 决定输出;由 OutputPlan 决定
	}
	if configPath != nil && *configPath != "" {
		if opt, err := readConfig(*configPath); err == nil {
			options = *opt
		}
	}
	outputSpec := "json"
	if cmdOpts != nil {
		if cmdOpts.Timeout > 0 {
			options.Timeout = time.Duration(cmdOpts.Timeout) * time.Second
		}
		if cmdOpts.Concurrency > 0 {
			options.Concurrency = cmdOpts.Concurrency
		}
		if cmdOpts.DownloadURL != "" {
			options.DownloadURL = cmdOpts.DownloadURL
		}
		if cmdOpts.DownloadSize != "" {
			options.DownloadSize = cmdOpts.DownloadSize
		}
		if cmdOpts.Threads > 0 {
			options.Threads = cmdOpts.Threads
			if options.Threads > 256 {
				options.Threads = 256
			}
		}
		switch cmdOpts.Mode {
		case "pingonly":
			options.SpeedTestMode = PingOnly
		case "speedonly":
			options.SpeedTestMode = SpeedOnly
		default:
			options.SpeedTestMode = "all"
		}
		if cmdOpts.Output != "" {
			outputSpec = cmdOpts.Output
		}
	}
	// check url / file
	if len(subscription) > 0 && subscription != options.Subscription {
		if _, err := url.Parse(subscription); err == nil {
			options.Subscription = subscription
		} else if _, err := os.Stat(subscription); err == nil {
			options.Subscription = subscription
		}
	}

	var outputFile, outputPic string
	if cmdOpts != nil {
		outputFile = cmdOpts.OutputFile
		outputPic = cmdOpts.OutputPicPath
	}
	targets, err := ParseOutputPlan(outputSpec, outputFile, outputPic, time.Now())
	if err != nil {
		return err
	}
	// 若计划含 pic,把路径写入 OutputPicPath 以触发 run() 内的公网 IP 抓取(供图片页脚)。
	for _, t := range targets {
		if t.Format == "pic" {
			options.OutputPicPath = t.Path
			break
		}
	}

	if jsonOpt, err := json.Marshal(options); err == nil {
		slog.Debug("cmd options", "json", string(jsonOpt))
	}

	links, err := ParseLinks(options.Subscription)
	if err != nil {
		return err
	}
	var writer MessageWriter = &EmptyMessageWriter{}
	if cmdOpts == nil || !cmdOpts.Silent {
		writer = NewCMDProgressWriter(os.Stderr)
	}
	p := &ProfileTest{
		Writer:      writer,
		Options:     &options,
		MessageType: 1,
		Links:       links,
	}
	_, summary, err := p.run(ctx)
	if err != nil {
		return err
	}
	return p.emitCMD(summary, targets)
}
```

- [ ] **Step 3: 构建 + 全量测试**

Run: `go build ./... && go test ./...`
Expected: PASS（`outputJSON` 已删除,无引用;`render` import 仍被 server.go 其它函数使用）

- [ ] **Step 4: 提交**

```bash
git add web/server.go web/profile.go
git commit -m "feat(cli): route TestFromCMD through OutputPlan + run + emitCMD"
```

---

### Task 8: main.go flags 更新与别名

**Files:**
- Modify: `main.go`（29-36 行 flag 定义;66-76、93-103 行两处 `CMDOptions` 构造）

- [ ] **Step 1: 更新 flag 说明 + 别名 + Silent** — 修改 `main.go`：

把 `output` 的说明改为逗号列表；新增 `init()` 注册 `-o`/`-f` 别名；两处 `CMDOptions` 加 `Silent: *logLevel == "silent"`。

将 29 行改为：
```go
	output       = flag.String("output", "json", "output formats (comma-separated): json, csv, text, table, pic, none")
```

在 `var (...)` 块之后、`fatal` 之前，新增：
```go
func init() {
	flag.StringVar(output, "o", "json", "alias for -output")
	flag.StringVar(outputFile, "f", "", "alias for -output-file")
}
```

两处 `cmdOpts := &webServer.CMDOptions{...}` 均在 `Mode: *mode,` 之后加一行：
```go
		Mode:          *mode,
		Silent:        *logLevel == "silent",
```

- [ ] **Step 2: 构建**

Run: `go build -o /tmp/pst . && /tmp/pst -h 2>&1 | grep -E '\-o |\-output |\-f |csv'`
Expected: 输出含 `-o`、`-output`（含 csv/table 说明）、`-f`

- [ ] **Step 3: 冒烟测试(错误路径,无需联网)**

Run: `/tmp/pst --test ss://invalid --output bogus 2>&1; echo "exit=$?"`
Expected: `unknown output format "bogus"` + `exit=1`

- [ ] **Step 4: 提交**

```bash
git add main.go
git commit -m "feat(cli): comma-list -output usage + -o/-f aliases + Silent"
```

---

### Task 9: 文档更新（README）

**Files:**
- Modify: `README.md`（Usage / options 表 / 删除 proxy 章节 99-102 行区域）

- [ ] **Step 1: 更新 README** — 完成以下改动：
  1. `Command line options` 表中：`-output` 行改为 `Output formats (comma-separated): json, csv, text, table, pic, none`；新增 `-output-file`/`-o`/`-f`/`-output-pic` 行。
  2. `Run as a speed test tool` 增补示例：
     ```bash
     # 单个分享链接直接测速
     ./proxy-speedtest "vmess://..."
     # CSV 到标准输出(可管道)
     ./proxy-speedtest --test https://sub --output csv > result.csv
     # 一次产出 JSON + 图片
     ./proxy-speedtest --test https://sub --output json,pic --output-pic result.png
     # 终端可读表格
     ./proxy-speedtest --test https://sub --output table
     ```
  3. 新增「Output formats」小节：说明 6 种格式、stdout/文件落盘规则、时间戳默认文件名、CSV 列。
  4. **删除**「Run as a HTTP/SOCKS5 proxy」整节（现单链接=直接测速,不再起代理）。

- [ ] **Step 2: 提交**

```bash
git add README.md
git commit -m "docs: document CLI multi-format output, csv/table, remove stale proxy section"
```

---

### Task 10: 端到端验证 + 收尾

- [ ] **Step 1: 全量测试 + 构建**

Run: `go vet ./... && go test ./... && go build -o /tmp/pst .`
Expected: 全绿

- [ ] **Step 2: 用本地 clash/profiles 小样本跑真实一次**（若手头有可用订阅/节点文件）

Run: `/tmp/pst --test <本地节点文件或订阅> --output table,csv,pic --output-pic /tmp/r.png --timeout 12 --concurrency 4`
Expected: stderr 有逐节点进度;生成 `/tmp/r.png` 与自动命名 csv;table 打到终端。若无可用节点,验证「no profile found」非零退出与错误路径即可。

- [ ] **Step 3: 确认无残留脏文件** — 确认运行 json 模式不再产生多余 `output.json`；`-output pic` 不再产生 `out.png`。

- [ ] **Step 4: 合并到 main 并打 tag（版本号先与用户确认）**

```bash
git checkout main && git merge --no-ff feat/cli-multi-output -m "feat: CLI multi-format output (csv/table/multi/progress)"
# 版本号确认后:
git tag v1.7.0 && git push origin main && git push origin v1.7.0
```

## Self-Review 记录

- 覆盖：CSV(T2)、table(T3)、多格式/落盘(T1)、stderr 进度(T4)、run 拆分(T5)、emit(T6)、TestFromCMD 重写(T7)、别名/flags(T8)、README+删proxy(T9)、验证+tag(T10)。逐条对应设计文档。
- 类型一致：`TestSummary`/`OutputTarget`/`nodeProgress` 命名在各任务一致；`render.Node.Id`（非 ID）与 `render.Nodes.Sort` 与源码一致。
- 无占位符：所有代码步骤含完整代码。
