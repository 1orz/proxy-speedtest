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

// formatProgressLine 生成单节点进度行:成功(有 ping>0 或有效下行速度)显示 ✓ + 指标,否则 ✗ failed。
func formatProgressLine(done, total int, np nodeProgress) string {
	ok := (np.pingSet && np.ping > 0) || (np.down != "" && np.down != "N/A")
	status := "✗"
	metrics := "failed"
	if ok {
		status = "✓"
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
// 实现 MessageWriter,stdout 因此可保持纯净(仅数据结果),便于管道。
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
