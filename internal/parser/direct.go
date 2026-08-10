package parser

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

// DirectLink 是「直连(不经任何代理)」的合成链接。
//
// 它的存在让「不填订阅、只测本机上传/下载带宽」这件事复用整条既有测速流水线
// (解析 → 拨号 → ping/下载/上传 → 出结果),而不必在每一层再加一套无代理分支:
// direct:// 解析成一个 freedom 出站的 ProxyConfig,对上层就是「一个普通节点」。
//
// 可选的 `#备注` 片段覆盖显示名,例如 direct://#家宽。
const DirectLink = "direct://"

// DirectTag 是直连节点的默认显示名。
const DirectTag = "Direct"

var regDirectLink = regexp.MustCompile(`(?i)^direct://`)

// IsDirectLink 判断一段文本是否为直连链接(忽略首尾空白,协议名大小写不敏感)。
func IsDirectLink(link string) bool {
	return regDirectLink.MatchString(strings.TrimSpace(link))
}

// Direct 解析 direct:// 链接。与其它解析器不同,它没有服务器地址/端口 —— 拨号时
// 走 xray 的 freedom 出站,即操作系统的默认出口。
type Direct struct{}

func (d *Direct) CanParse(link string) bool {
	return IsDirectLink(link)
}

func (d *Direct) Parse(link string) (*xray.ProxyConfig, error) {
	link = strings.TrimSpace(link)
	if !IsDirectLink(link) {
		return nil, fmt.Errorf("not a direct link: %s", truncateLink(link))
	}
	tag := DirectTag
	if _, fragment, ok := strings.Cut(link, "#"); ok {
		if unescaped, err := url.QueryUnescape(fragment); err == nil {
			fragment = unescaped
		}
		if name := strings.TrimSpace(fragment); name != "" {
			tag = name
		}
	}
	return &xray.ProxyConfig{
		Protocol: xray.ProtocolDirect,
		Tag:      tag,
		Link:     link,
	}, nil
}
