package web

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/1orz/proxy-speedtest/download"
	"github.com/1orz/proxy-speedtest/web/render"
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
// 规则:数据类(json/csv/text/table)未给文件且仅一种 -> stdout;多种或与文件配合 ->
// 首个数据类用文件、其余按时间戳自动命名;图片类(pic)永远落盘。
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

// TestSummary 承载一次测速的最终结果与汇总(供 CLI emitter 使用)。
type TestSummary struct {
	Nodes        render.Nodes
	Traffic      int64
	Duration     string
	SuccessCount int
	LinksCount   int
}

// renderCSVBytes 把结果渲染为 CSV(机器友好:速度为字节/秒整数,与 JSON 一致)。
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

// renderTextBytes 输出测通节点的分享链接(每行一条,可当订阅导入)。
func renderTextBytes(summary TestSummary) []byte {
	var links []string
	for _, n := range summary.Nodes {
		if n.Ping != "0" || n.AvgSpeed > 0 || n.MaxSpeed > 0 {
			links = append(links, n.Link)
		}
	}
	return []byte(strings.Join(links, "\n"))
}

// renderJSONBytes 输出与 web JSONOutput 一致的缩进 JSON。
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

// truncateRunes 按 rune 截断,超出则以 … 结尾。
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

// renderTableString 生成终端可读对齐表 + 底部汇总行。失败节点 Ping/Down/Up 显示 -。
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

// emitCMD 按 targets 渲染并输出。数据类节点先按 SortMethod 排序(副本,不动 summary);
// 空 Path 写 stdout,否则写文件。图片类经 savePicPath 落盘。任一失败即返回错误(→非零退出)。
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

// savePicPath 把结果渲染为 PNG 到指定路径(与 savePic 同,但路径显式传入)。
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
