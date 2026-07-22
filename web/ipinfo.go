package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// PublicIP 是测速机某个地址族(v4/v6)的公网出口信息:发起测速的机器直连公网时
// 对外呈现的地址与归属地。用于图片页脚与 GUI 展示,best-effort,失败即空。
type PublicIP struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
	ISP     string `json:"isp"`
}

// Geo 返回紧凑归属地串 "国家, 城市, ISP"(缺省字段跳过)。
func (p *PublicIP) Geo() string {
	if p == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	for _, s := range []string{p.Country, p.City, p.ISP} {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

// Line 返回图片页脚整行,如 "IPv4: 1.2.3.4 (US, Los Angeles, Cloudflare)"。
// IP 为空时返回空串(不占位)。
func (p *PublicIP) Line(label string) string {
	if p == nil || p.IP == "" {
		return ""
	}
	if geo := p.Geo(); geo != "" {
		return fmt.Sprintf("%s: %s (%s)", label, p.IP, geo)
	}
	return fmt.Sprintf("%s: %s", label, p.IP)
}

// familyClient 构造一个把连接强制到指定地址族("tcp4"/"tcp6")的 http.Client,
// 从而让返回自连接 IP 的服务(ip-api/cip.cc 等)报告对应族的出口地址。
func familyClient(network string, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		},
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
	}
	return &http.Client{Timeout: timeout, Transport: tr}
}

type ipProvider struct {
	name  string
	fetch func(ctx context.Context, c *http.Client) (*PublicIP, error)
}

func httpGet(ctx context.Context, c *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// 部分服务(cip.cc)对浏览器 UA 返回 HTML,对 curl 类 UA 返回纯文本。
	req.Header.Set("User-Agent", "curl/8.4.0")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64*1024))
}

// providers 按优先级排列;逐个尝试直到取到有效结果(IP 非空且族匹配)。
var providers = []ipProvider{
	{name: "ip-api.com", fetch: func(ctx context.Context, c *http.Client) (*PublicIP, error) {
		body, err := httpGet(ctx, c, "http://ip-api.com/json/?fields=status,country,regionName,city,isp,query")
		if err != nil {
			return nil, err
		}
		var r struct {
			Status     string `json:"status"`
			Country    string `json:"country"`
			RegionName string `json:"regionName"`
			City       string `json:"city"`
			ISP        string `json:"isp"`
			Query      string `json:"query"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		if r.Status != "success" || r.Query == "" {
			return nil, fmt.Errorf("ip-api: no result")
		}
		return &PublicIP{IP: r.Query, Country: r.Country, Region: r.RegionName, City: r.City, ISP: r.ISP}, nil
	}},
	{name: "ip.sb", fetch: func(ctx context.Context, c *http.Client) (*PublicIP, error) {
		body, err := httpGet(ctx, c, "https://api.ip.sb/geoip")
		if err != nil {
			return nil, err
		}
		var r struct {
			IP           string `json:"ip"`
			Country      string `json:"country"`
			Region       string `json:"region"`
			City         string `json:"city"`
			ISP          string `json:"isp"`
			Organization string `json:"organization"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		if r.IP == "" {
			return nil, fmt.Errorf("ip.sb: no result")
		}
		isp := r.ISP
		if isp == "" {
			isp = r.Organization
		}
		return &PublicIP{IP: r.IP, Country: r.Country, Region: r.Region, City: r.City, ISP: isp}, nil
	}},
	{name: "ipinfo.io", fetch: func(ctx context.Context, c *http.Client) (*PublicIP, error) {
		body, err := httpGet(ctx, c, "https://ipinfo.io/json")
		if err != nil {
			return nil, err
		}
		var r struct {
			IP      string `json:"ip"`
			City    string `json:"city"`
			Region  string `json:"region"`
			Country string `json:"country"`
			Org     string `json:"org"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		if r.IP == "" {
			return nil, fmt.Errorf("ipinfo: no result")
		}
		return &PublicIP{IP: r.IP, Country: r.Country, Region: r.Region, City: r.City, ISP: r.Org}, nil
	}},
	{name: "cip.cc", fetch: func(ctx context.Context, c *http.Client) (*PublicIP, error) {
		body, err := httpGet(ctx, c, "https://www.cip.cc/")
		if err != nil {
			return nil, err
		}
		return parseCipCC(string(body))
	}},
}

// parseCipCC 解析 cip.cc 的纯文本响应:
//
//	IP      : 1.2.3.4
//	地址     : 中国  北京
//	运营商    : 联通
func parseCipCC(text string) (*PublicIP, error) {
	if strings.Contains(text, "<html") || strings.Contains(text, "<!DOCTYPE") {
		return nil, fmt.Errorf("cip.cc: html response")
	}
	p := &PublicIP{}
	for _, line := range strings.Split(text, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		switch k {
		case "IP":
			p.IP = v
		case "地址":
			// "中国  北京" -> Country + City
			fields := strings.Fields(v)
			if len(fields) > 0 {
				p.Country = fields[0]
			}
			if len(fields) > 1 {
				p.City = strings.Join(fields[1:], " ")
			}
		case "运营商":
			p.ISP = v
		}
	}
	if p.IP == "" {
		return nil, fmt.Errorf("cip.cc: no ip")
	}
	return p, nil
}

// familyMatch 校验取到的 IP 属于期望的地址族,防止某来源无视连接族回报了另一族地址。
func familyMatch(ipStr, network string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	if network == "tcp6" {
		return ip.To4() == nil
	}
	return ip.To4() != nil
}

// fetchPublicIP 在指定地址族上逐个尝试来源,返回第一个有效结果。
func fetchPublicIP(ctx context.Context, network string) (*PublicIP, error) {
	client := familyClient(network, 3*time.Second)
	var lastErr error
	for _, pv := range providers {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		info, err := pv.fetch(ctx, client)
		if err != nil {
			lastErr = err
			slog.Debug("public ip provider failed", "provider", pv.name, "network", network, "err", err)
			continue
		}
		if !familyMatch(info.IP, network) {
			lastErr = fmt.Errorf("%s: ip %s not %s", pv.name, info.IP, network)
			continue
		}
		slog.Debug("public ip resolved", "provider", pv.name, "network", network, "ip", info.IP)
		return info, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no provider succeeded")
	}
	return nil, lastErr
}

// FetchBoth 并发抓取 v4/v6 出口信息(best-effort;某族不可用则该返回值为 nil)。
// 整体受 ctx 约束,内部再叠加一个上限,避免个别来源挂起拖住渲染。
func FetchBoth(ctx context.Context) (v4 *PublicIP, v6 *PublicIP) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	done := make(chan struct{}, 2)
	go func() {
		if info, err := fetchPublicIP(ctx, "tcp4"); err == nil {
			v4 = info
		}
		done <- struct{}{}
	}()
	go func() {
		if info, err := fetchPublicIP(ctx, "tcp6"); err == nil {
			v6 = info
		}
		done <- struct{}{}
	}()
	<-done
	<-done
	return v4, v6
}
