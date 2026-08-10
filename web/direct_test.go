package web

import (
	"encoding/json"
	"testing"

	"github.com/1orz/proxy-speedtest/config"
	"github.com/1orz/proxy-speedtest/web/render"
)

// 前端一条订阅都没填时发来的载荷:应退化为「直连测速」,而不是报 invalidsub。
func TestParseMessageDirectWhenNoSubscription(t *testing.T) {
	payloads := map[string]string{
		"explicit flag":     `{"testMode":2,"directMode":true,"subscriptions":[],"speedtestMode":"all","timeout":15}`,
		"empty list":        `{"testMode":2,"subscriptions":[],"speedtestMode":"all","timeout":15}`,
		"blank entries":     `{"testMode":2,"subscriptions":[{"group":"","subscription":"   "}],"timeout":15}`,
		"no subscription":   `{"testMode":2,"timeout":15}`,
		"blank single sub":  `{"testMode":2,"subscription":"  ","timeout":15}`,
		"direct link typed": `{"testMode":2,"subscriptions":[{"group":"","subscription":"direct://"}],"timeout":15}`,
	}
	for name, payload := range payloads {
		links, _, options, err := parseMessage([]byte(payload))
		if err != nil {
			t.Fatalf("%s: parseMessage failed: %v", name, err)
		}
		if len(links) != 1 || !config.IsDirectLink(links[0]) {
			t.Fatalf("%s: links = %v, want one direct link", name, links)
		}
		if options.Timeout < 1 {
			t.Fatalf("%s: timeout not normalised: %v", name, options.Timeout)
		}
	}
}

// 显式勾选直连时,即便填了订阅也以直连为准(前端只在没订阅时才置位,这里锁定语义)。
func TestParseMessageDirectFlagWins(t *testing.T) {
	payload := `{"testMode":2,"directMode":true,"subscriptions":[{"group":"g","subscription":"https://example.com/sub"}],"timeout":15}`
	links, groups, options, err := parseMessage([]byte(payload))
	if err != nil {
		t.Fatalf("parseMessage failed: %v", err)
	}
	if len(links) != 1 || !config.IsDirectLink(links[0]) {
		t.Fatalf("links = %v, want one direct link", links)
	}
	if groups != nil {
		t.Fatalf("groups = %v, want nil", groups)
	}
	if !options.DirectMode {
		t.Fatal("DirectMode should stay set")
	}
}

// 填了订阅但解析不出节点,必须照旧报错 —— 不能悄悄降级成直连测速,
// 否则用户会以为自己测的是代理。
func TestParseMessageUnparsableSubscriptionStillErrors(t *testing.T) {
	payload := `{"testMode":2,"subscriptions":[{"group":"","subscription":"definitely not a subscription"}],"timeout":15}`
	if _, _, _, err := parseMessage([]byte(payload)); err == nil {
		t.Fatal("parseMessage succeeded, want error")
	}
}

func TestParseLinksDirect(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"direct://", "direct://"},
		{"  direct://  ", "direct://"},
		{"DIRECT://#家宽", "DIRECT://#家宽"},
	} {
		links, err := ParseLinks(tc.in)
		if err != nil {
			t.Fatalf("ParseLinks(%q) failed: %v", tc.in, err)
		}
		if len(links) != 1 || links[0] != tc.want {
			t.Fatalf("ParseLinks(%q) = %v, want [%q]", tc.in, links, tc.want)
		}
	}
}

// text 输出号称「可当订阅导入」,合成的 direct:// 不是节点,必须被排除。
func TestWorkingLinksSkipsDirect(t *testing.T) {
	nodes := render.Nodes{
		{Id: 0, Link: "direct://", Ping: "20", AvgSpeed: 1 << 20},
		{Id: 1, Link: "vmess://ok", Ping: "30", AvgSpeed: 1 << 20},
		{Id: 2, Link: "vmess://dead", Ping: "0"},
	}
	got := workingLinks(nodes)
	if len(got) != 1 || got[0] != "vmess://ok" {
		t.Fatalf("workingLinks = %v, want [vmess://ok]", got)
	}
	if len(renderTextBytes(TestSummary{Nodes: nodes})) != len("vmess://ok") {
		t.Fatalf("renderTextBytes = %q", renderTextBytes(TestSummary{Nodes: nodes}))
	}
}

// 直连没有服务器地址:server 字段应省略,而不是发出 ":0"。
func TestGotServersMsgDirect(t *testing.T) {
	var msg Message
	if err := json.Unmarshal(gotserversMsg(0, []string{config.DirectLink}, nil), &msg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(msg.Servers) != 1 {
		t.Fatalf("servers = %v, want 1", msg.Servers)
	}
	server := msg.Servers[0]
	if server.Server != "" {
		t.Fatalf("server = %q, want empty", server.Server)
	}
	if server.Protocol != "direct" {
		t.Fatalf("protocol = %q, want direct", server.Protocol)
	}
	if server.Remarks == "" {
		t.Fatal("remarks should not be empty")
	}
}

// 首行是 direct:// 的多行列表仍按普通列表解析,不会被直连短路整体吞掉。
func TestParseLinksDirectDoesNotSwallowList(t *testing.T) {
	links, err := ParseLinks("direct://\nvmess://ignored\ntrojan://user@example.com:443#n")
	if err != nil {
		t.Fatalf("ParseLinks failed: %v", err)
	}
	for _, l := range links {
		if config.IsDirectLink(l) {
			t.Fatalf("direct link leaked into a multi-line list: %v", links)
		}
	}
	if len(links) == 0 {
		t.Fatal("real links were dropped")
	}
}
