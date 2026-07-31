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

func TestFormatLiveProgressLine(t *testing.T) {
	down := formatLiveProgressLine(2, 12, nodeProgress{remarks: "Tokyo"}, "down", "20.0MB", "30.0MB", 25*1024*1024)
	for _, want := range []string{"[2/12]", "Tokyo", "↓ 25.0MB/s", "avg 20.0MB", "max 30.0MB"} {
		if !strings.Contains(down, want) {
			t.Fatalf("download line missing %q: %q", want, down)
		}
	}
	up := formatLiveProgressLine(2, 12, nodeProgress{remarks: "Tokyo"}, "up", "6.0MB", "8.0MB", 7*1024*1024)
	if !strings.Contains(up, "↑ 7.0MB/s") {
		t.Fatalf("upload line wrong: %q", up)
	}
}

func TestCMDProgressWriterFlow(t *testing.T) {
	var buf bytes.Buffer
	w := NewCMDProgressWriter(&buf)
	_ = w.WriteMessage(1, []byte(`{"id":0,"info":"gotservers","servers":[{"id":0,"info":"gotserver","remarks":"HK","protocol":"vmess"},{"id":1,"info":"gotserver","remarks":"JP","protocol":"ss"}]}`))
	_ = w.WriteMessage(1, []byte(`{"id":0,"info":"gotping","ping":42}`))
	_ = w.WriteMessage(1, []byte(`{"id":0,"info":"gotspeed","speed":"12.3MB","maxspeed":"15.0MB","traffic":10485760}`))
	_ = w.WriteMessage(1, []byte(`{"id":0,"info":"gotupload","uploadspeed":"3.0MB","maxuploadspeed":"4.0MB","traffic":2097152}`))
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
	if !strings.Contains(out, "↓ 10.0MB/s") || !strings.Contains(out, "↑ 2.0MB/s") {
		t.Fatalf("progress out missing live samples: %q", out)
	}
}
