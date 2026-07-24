package web

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/1orz/proxy-speedtest/web/render"
)

func fixedNow() time.Time { return time.Date(2026, 7, 24, 15, 30, 5, 0, time.UTC) }

func TestParseOutputPlan(t *testing.T) {
	ts := "speedtest-20260724-153005"
	cases := []struct {
		name            string
		spec, file, pic string
		want            []OutputTarget
		wantErr         bool
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
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("len=%d want=%d (%v)", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("target[%d]=%+v want %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}

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
