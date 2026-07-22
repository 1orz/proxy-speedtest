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
	"sync"
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

// Geo 返回紧凑归属地串 "国家, 省/区, 城市, ISP"(缺省与重复字段跳过)。
func (p *PublicIP) Geo() string {
	if p == nil {
		return ""
	}
	var parts []string
	seen := map[string]bool{}
	for _, s := range []string{p.Country, p.Region, p.City, p.ISP} {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

// merge 用 src 的非空字段补齐 dst 的缺失字段(dst 已有值优先,不覆盖)。
func (p *PublicIP) merge(src *PublicIP) {
	if src == nil {
		return
	}
	if p.IP == "" {
		p.IP = src.IP
	}
	if p.Country == "" {
		p.Country = src.Country
	}
	if p.Region == "" {
		p.Region = src.Region
	}
	if p.City == "" {
		p.City = src.City
	}
	if p.ISP == "" {
		p.ISP = src.ISP
	}
}

// Line 返回图片页脚整行,如 "IPv4: 1.2.3.4 (US, Los Angeles, Cloudflare)"。
// IP 为空时返回空串(不占位)。
func (p *PublicIP) Line(label string) string {
	if p == nil {
		return ""
	}
	return ipDisplayLine(label, p.IP, p.Geo())
}

// ipDisplayLine 由 (标签, IP, 归属地) 拼出展示行;IP 为空返回空串(不占位)。
// 供前端"重新生成图片"端点复用(那里传的是已拆分的 ip/geo 字段)。
func ipDisplayLine(label, ip, geo string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	if geo = strings.TrimSpace(geo); geo != "" {
		return fmt.Sprintf("%s: %s (%s)", label, ip, geo)
	}
	return fmt.Sprintf("%s: %s", label, ip)
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

// providers 按优先级排列;并发查询后按此顺序合并(靠前者字段冲突时优先)。
// ipip.net 置顶:对国内 IP 的省/市/运营商更准更细(中文)。
var providers = []ipProvider{
	{name: "ipip.net", fetch: func(ctx context.Context, c *http.Client) (*PublicIP, error) {
		body, err := httpGet(ctx, c, "https://myip.ipip.net/")
		if err != nil {
			return nil, err
		}
		return parseIPIP(string(body))
	}},
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
	{name: "ipwho.is", fetch: func(ctx context.Context, c *http.Client) (*PublicIP, error) {
		body, err := httpGet(ctx, c, "https://ipwho.is/")
		if err != nil {
			return nil, err
		}
		var r struct {
			IP         string `json:"ip"`
			Success    bool   `json:"success"`
			Country    string `json:"country"`
			Region     string `json:"region"`
			City       string `json:"city"`
			Connection struct {
				ISP string `json:"isp"`
				Org string `json:"org"`
			} `json:"connection"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		if !r.Success || r.IP == "" {
			return nil, fmt.Errorf("ipwho.is: no result")
		}
		isp := r.Connection.ISP
		if isp == "" {
			isp = r.Connection.Org
		}
		return &PublicIP{IP: r.IP, Country: r.Country, Region: r.Region, City: r.City, ISP: isp}, nil
	}},
	{name: "ipapi.co", fetch: func(ctx context.Context, c *http.Client) (*PublicIP, error) {
		body, err := httpGet(ctx, c, "https://ipapi.co/json/")
		if err != nil {
			return nil, err
		}
		var r struct {
			IP          string `json:"ip"`
			City        string `json:"city"`
			Region      string `json:"region"`
			CountryName string `json:"country_name"`
			Org         string `json:"org"`
			Error       bool   `json:"error"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		if r.Error || r.IP == "" {
			return nil, fmt.Errorf("ipapi.co: no result")
		}
		return &PublicIP{IP: r.IP, Country: r.CountryName, Region: r.Region, City: r.City, ISP: r.Org}, nil
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

// parseIPIP 解析 myip.ipip.net 的纯文本:
//
//	当前 IP:61.175.246.226  来自于:中国 浙江 温州 电信
func parseIPIP(text string) (*PublicIP, error) {
	// 容错:把可能出现的全角冒号换成半角
	text = strings.TrimSpace(strings.ReplaceAll(text, "：", ":"))
	if strings.Contains(text, "<html") || strings.Contains(text, "<!DOCTYPE") {
		return nil, fmt.Errorf("ipip: html response")
	}
	p := &PublicIP{}
	if i := strings.Index(text, "IP:"); i >= 0 {
		if fs := strings.Fields(text[i+len("IP:"):]); len(fs) > 0 {
			p.IP = fs[0]
		}
	}
	if i := strings.Index(text, "来自于:"); i >= 0 {
		f := strings.Fields(text[i+len("来自于:"):])
		if len(f) > 0 {
			p.Country = f[0]
		}
		switch {
		case len(f) >= 4:
			p.Region, p.City, p.ISP = f[1], f[2], strings.Join(f[3:], " ")
		case len(f) == 3:
			p.City, p.ISP = f[1], f[2]
		case len(f) == 2:
			p.ISP = f[1]
		}
	}
	if p.IP == "" {
		return nil, fmt.Errorf("ipip: no ip")
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

// fetchPublicIP 在指定地址族上并发查询所有来源,再按 providers 优先级顺序合并,
// 把各源的国家/省区/城市/ISP 补齐成尽可能完整的一条(靠前来源字段冲突时优先)。
// 并发 → 快;合并所有源 → 更全更准(如国内 IP 由 ipip.net 提供省市)。
func fetchPublicIP(ctx context.Context, network string) (*PublicIP, error) {
	client := familyClient(network, 3*time.Second)
	results := make([]*PublicIP, len(providers))
	var wg sync.WaitGroup
	for i, pv := range providers {
		wg.Add(1)
		go func(i int, pv ipProvider) {
			defer wg.Done()
			info, err := pv.fetch(ctx, client)
			if err != nil {
				slog.Debug("public ip provider failed", "provider", pv.name, "network", network, "err", err)
				return
			}
			if !familyMatch(info.IP, network) {
				return
			}
			results[i] = info // 各 goroutine 写各自下标,无需加锁
		}(i, pv)
	}
	wg.Wait()

	acc := &PublicIP{}
	for _, r := range results {
		acc.merge(r)
	}
	if acc.IP == "" {
		return nil, fmt.Errorf("no provider succeeded")
	}
	slog.Debug("public ip resolved", "network", network, "ip", acc.IP, "geo", acc.Geo())
	return acc, nil
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
