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
