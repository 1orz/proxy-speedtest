package parser

import (
	"testing"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

func TestIsDirectLink(t *testing.T) {
	for _, link := range []string{"direct://", "DIRECT://", " direct:// ", "direct://#家宽"} {
		if !IsDirectLink(link) {
			t.Fatalf("IsDirectLink(%q) = false, want true", link)
		}
	}
	for _, link := range []string{"", "direct", "direct:/", "vmess://abc", "https://example.com"} {
		if IsDirectLink(link) {
			t.Fatalf("IsDirectLink(%q) = true, want false", link)
		}
	}
}

func TestParseLinkDirect(t *testing.T) {
	config, err := ParseLink(DirectLink)
	if err != nil {
		t.Fatalf("ParseLink(%q) failed: %v", DirectLink, err)
	}
	if config.Protocol != xray.ProtocolDirect {
		t.Fatalf("protocol = %q, want %q", config.Protocol, xray.ProtocolDirect)
	}
	if config.Tag != DirectTag {
		t.Fatalf("tag = %q, want %q", config.Tag, DirectTag)
	}
	// 直连没有服务器:上层据此把「服务器」一列留空,而不是显示成 ":0"。
	if config.Address != "" || config.Port != 0 {
		t.Fatalf("want no server address, got %q:%d", config.Address, config.Port)
	}
}

func TestParseLinkDirectCustomTag(t *testing.T) {
	for _, tc := range []struct {
		link string
		want string
	}{
		{"direct://#家宽", "家宽"},
		{"direct://#%E5%AE%B6%E5%AE%BD", "家宽"}, // 片段被 URL 转义时同样还原
		{"direct://#  ", DirectTag},           // 空白片段回退到默认名
		{"DIRECT://", DirectTag},
	} {
		config, err := ParseLink(tc.link)
		if err != nil {
			t.Fatalf("ParseLink(%q) failed: %v", tc.link, err)
		}
		if config.Tag != tc.want {
			t.Fatalf("ParseLink(%q).Tag = %q, want %q", tc.link, config.Tag, tc.want)
		}
	}
}

// direct:// 必须走 Direct 解析器,不能被其它协议解析器抢走。
func TestDirectDoesNotCollideWithOtherParsers(t *testing.T) {
	for _, p := range parsers {
		if _, isDirect := p.(*Direct); isDirect {
			continue
		}
		if p.CanParse(DirectLink) {
			t.Fatalf("%T also claims %q", p, DirectLink)
		}
	}
	if (&Direct{}).CanParse("vmess://abc") {
		t.Fatal("Direct claims a vmess link")
	}
}
