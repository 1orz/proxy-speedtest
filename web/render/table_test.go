package render

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"
)

func sampleNodes(n int, withUpload bool) Nodes {
	nodes := make([]Node, n)
	for i := 0; i < n; i++ {
		nodes[i] = Node{
			Group:    "节点列表",
			Remarks:  fmt.Sprintf("美国加利福尼亚免费测试%d", i),
			Protocol: "vmess/ws",
			Ping:     fmt.Sprintf("%d", rand.Intn(800-50)+50),
			AvgSpeed: int64((rand.Intn(20-1) + 1) * 1024 * 1024),
			MaxSpeed: int64((rand.Intn(60-5) + 5) * 1024 * 1024),
		}
		if withUpload {
			nodes[i].UploadSpeed = int64((rand.Intn(10-1) + 1) * 1024 * 1024)
			nodes[i].MaxUploadSpeed = nodes[i].UploadSpeed * 2
		}
	}
	return nodes
}

func TestDefaultTable(t *testing.T) {
	nodes := sampleNodes(50, false)
	fontPath, _ := filepath.Abs("../misc/WenQuanYiMicroHei-01.ttf")
	table, err := DefaultTable(nodes, fontPath)
	if err != nil {
		t.Error(err)
	}
	msg := table.FormatTraffic("10.2G", "3m13s", "50/50")
	table.Draw("out.png", msg)
}

// TestUploadDarkTable 冒烟测试:含上传列 + 深色主题 + IP 页脚,能构建并渲染不 panic。
func TestUploadDarkTable(t *testing.T) {
	nodes := sampleNodes(8, true)
	fontPath, _ := filepath.Abs("../misc/WenQuanYiMicroHei-01.ttf")
	options := NewTableOptions(40, 30, 0.5, 0.5, 24, 0.5, fontPath, "cn", "rainbow", "Asia/Shanghai", nil)
	options.SetAppearance("dark")
	options.SetIPInfo("IPv4: 1.2.3.4 (US, Los Angeles, Cloudflare)", "IPv6: 2606:4700::1111 (US, Los Angeles, Cloudflare)")
	table, err := NewTableWithOption(nodes, &options)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range table.columns {
		if c.key == "Upload" {
			found = true
		}
	}
	if !found {
		t.Error("expected Upload column when nodes have upload speed")
	}
	if _, err := table.Encode(table.FormatTraffic("2.2G", "1m03s", "8/8")); err != nil {
		t.Error(err)
	}
}

func TestCSV2Nodes(t *testing.T) {
	t.Skip("Skipping test that requires external CSV file")
	nodes, err := CSV2Nodes("/home/arch/Downloads/test.csv")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(nodes)
}
