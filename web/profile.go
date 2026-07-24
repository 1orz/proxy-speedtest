package web

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/1orz/proxy-speedtest/config"
	"github.com/1orz/proxy-speedtest/download"
	"github.com/1orz/proxy-speedtest/request"
	"github.com/1orz/proxy-speedtest/utils"
	"github.com/1orz/proxy-speedtest/web/render"
)

var (
	ErrInvalidData = errors.New("invalid data")
	regProfile     = regexp.MustCompile(`((?i)vmess://(\S+?)@(\S+?):([0-9]{2,5})/([?#][^\s]+))|((?i)vmess://[a-zA-Z0-9+_/=-]+([?#][^\s]+)?)|((?i)ssr://[a-zA-Z0-9+_/=-]+)|((?i)(vless|ss|trojan)://(\S+?)@(\S+?):([0-9]{2,5})/?([?#][^\s]+))|((?i)(ss)://[a-zA-Z0-9+_/=-]+([?#][^\s]+))`)
)

const (
	PIC_BASE64 = iota
	PIC_PATH
	PIC_NONE
	JSON_OUTPUT
	TEXT_OUTPUT
)

type PAESE_TYPE int

const (
	PARSE_ANY PAESE_TYPE = iota
	PARSE_URL
	PARSE_FILE
	PARSE_BASE64
	PARSE_CLASH
	PARSE_PROFILE
)

// support proxy
// concurrency setting
// as subscription server
// profiles filter
// clash to vmess local subscription
func getSubscriptionLinks(link string) ([]string, error) {
	c := http.Client{
		Timeout: 20 * time.Second,
	}
	resp, err := c.Get(link)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if isYamlFile(link) {
		return scanClashProxies(resp.Body, true)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	dataStr := string(data)
	msg, err := utils.DecodeB64(dataStr)
	if err != nil {
		if strings.Contains(dataStr, "proxies:") {
			return parseClash(dataStr)
		} else if strings.Contains(dataStr, "vmess://") ||
			strings.Contains(dataStr, "trojan://") ||
			strings.Contains(dataStr, "ssr://") ||
			strings.Contains(dataStr, "ss://") {
			return parseProfiles(dataStr)
		} else {
			return []string{}, err
		}
	}
	return ParseLinks(msg)
}

type parseFunc func(string) ([]string, error)

type ParseOption struct {
	Type PAESE_TYPE
}

// api
func ParseLinks(message string) ([]string, error) {
	opt := ParseOption{Type: PARSE_ANY}
	return ParseLinksWithOption(message, opt)
}

// api
func ParseLinksWithOption(message string, opt ParseOption) ([]string, error) {
	// matched, err := regexp.MatchString(`^(?:https?:\/\/)(?:[^@\/\n]+@)?(?:www\.)?([^:\/\n]+)`, message)
	if opt.Type == PARSE_URL || utils.IsUrl(message) {
		slog.Debug("parsing subscription url", "url", message)
		return getSubscriptionLinks(message)
	}
	// check is file path
	if opt.Type == PARSE_FILE || utils.IsFilePath(message) {
		return parseFile(message)
	}
	if opt.Type == PARSE_BASE64 {
		return parseBase64(message)
	}
	if opt.Type == PARSE_CLASH {
		return parseClash(message)
	}
	if opt.Type == PARSE_PROFILE {
		return parseProfiles(message)
	}
	var links []string
	var err error
	for _, fn := range []parseFunc{parseProfiles, parseBase64, parseClash, parseFile} {
		links, err = fn(message)
		if err == nil && len(links) > 0 {
			break
		}
	}
	return links, err
}

func parseProfiles(data string) ([]string, error) {
	// encodeed url
	links := strings.Split(data, "\n")
	if len(links) > 1 {
		for i, link := range links {
			if l, err := url.Parse(link); err == nil {
				if query, err := url.QueryUnescape(l.RawQuery); err == nil && query == l.RawQuery {
					links[i] = l.String()
				}
			}
		}
		data = strings.Join(links, "\n")
	}
	// reg := regexp.MustCompile(`((?i)vmess://(\S+?)@(\S+?):([0-9]{2,5})/([?#][^\s]+))|((?i)vmess://[a-zA-Z0-9+_/=-]+([?#][^\s]+)?)|((?i)ssr://[a-zA-Z0-9+_/=-]+)|((?i)(vless|ss|trojan)://(\S+?)@(\S+?):([0-9]{2,5})([?#][^\s]+))|((?i)(ss)://[a-zA-Z0-9+_/=-]+([?#][^\s]+))`)
	matches := regProfile.FindAllStringSubmatch(data, -1)
	linksLen, matchesLen := len(links), len(matches)
	if linksLen < matchesLen {
		links = make([]string, matchesLen)
	} else if linksLen > matchesLen {
		links = links[:len(matches)]
	}
	for index, match := range matches {
		link := match[0]
		if config.RegShadowrocketVmess.MatchString(link) {
			if l, err := config.ShadowrocketLinkToVmessLink(link); err == nil {
				link = l
			}
		}
		links[index] = link
	}
	return links, nil
}

func parseBase64(data string) ([]string, error) {
	msg, err := utils.DecodeB64(data)
	if err != nil {
		return nil, err
	}
	return parseProfiles(msg)
}

func parseClash(data string) ([]string, error) {
	cc, err := config.ParseClash(utils.UnsafeGetBytes(data))
	if err != nil {
		return parseClashProxies(data)
	}
	return cc.Proxies, nil
}

// split to new line
func parseClashProxies(input string) ([]string, error) {

	if !strings.Contains(input, "{") {
		return []string{}, nil
	}
	return scanClashProxies(strings.NewReader(input), true)
}

func scanClashProxies(r io.Reader, greedy bool) ([]string, error) {
	proxiesStart := false
	var data []byte
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		b := scanner.Bytes()
		trimLine := strings.TrimSpace(string(b))
		if trimLine == "proxy-groups:" || trimLine == "rules:" || trimLine == "Proxy Group:" {
			break
		}
		if !proxiesStart && (trimLine == "proxies:" || trimLine == "Proxy:") {
			proxiesStart = true
			b = []byte("proxies:")
		}
		if proxiesStart {
			if _, err := config.ParseBaseProxy(trimLine); err != nil {
				continue
			}
			data = append(data, b...)
			data = append(data, byte('\n'))
		}
	}
	// fmt.Println(string(data))
	return parseClashByte(data)
}

func parseClashFileByLine(filepath string) ([]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return scanClashProxies(file, false)
}

func parseClashByte(data []byte) ([]string, error) {
	cc, err := config.ParseClash(data)
	if err != nil {
		return nil, err
	}
	return cc.Proxies, nil
}

func parseFile(filepath string) ([]string, error) {
	filepath = strings.TrimSpace(filepath)
	if _, err := os.Stat(filepath); err != nil {
		return nil, err
	}
	// clash
	if isYamlFile(filepath) {
		return parseClashFileByLine(filepath)
	}
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	links, err := parseBase64(string(data))
	if err != nil && len(data) > 2048 {
		preview := string(data[:2048])
		if strings.Contains(preview, "proxies:") {
			return scanClashProxies(bytes.NewReader(data), true)
		}
		if strings.Contains(preview, "vmess://") ||
			strings.Contains(preview, "trojan://") ||
			strings.Contains(preview, "ssr://") ||
			strings.Contains(preview, "ss://") {
			return parseProfiles(string(data))
		}
	}
	return links, err
}

const (
	SpeedOnly = "speedonly"
	PingOnly  = "pingonly"
	ALLTEST   = iota
	RETEST
)

// SubEntry 是一条「订阅链接 + 组名」输入。组名可空(空则回退到默认组名)。
type SubEntry struct {
	Group        string `json:"group"`
	Subscription string `json:"subscription"`
}

// HeaderKV 是一条用户自定义请求头(应用到下载/上传请求)。Name 为空则忽略。
type HeaderKV struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ProfileTestOptions struct {
	GroupName       string        `json:"group"`
	Subscriptions   []SubEntry    `json:"subscriptions"` // 多订阅(各带可选组名);非空时优先于 Subscription
	SpeedTestMode   string        `json:"speedtestMode"` // speedonly pingonly all
	SortMethod      string        `json:"sortMethod"`    // speed rspeed ping rping
	Concurrency     int           `json:"concurrency"`
	TestMode        int           `json:"testMode"` // 2: ALLTEST 3: RETEST
	TestIDs         []int         `json:"testids"`
	Timeout         time.Duration `json:"timeout"`
	Links           []string      `json:"links"`
	Subscription    string        `json:"subscription"`
	Language        string        `json:"language"`
	FontSize        int           `json:"fontSize"`
	Theme           string        `json:"theme"`
	Unique          bool          `json:"unique"`
	GeneratePicMode int           `json:"generatePicMode"` // 0: base64 1:pic path 2: no pic 3: json @deprecated use outputMode
	OutputMode      int           `json:"outputMode"`
	OutputFilePath  string        `json:"outputFilePath,omitempty"` // output file path for JSON result
	OutputPicPath   string        `json:"outputPicPath,omitempty"`  // output pic path (can be used with JSON output)
	DownloadURL     string        `json:"downloadUrl"`              // custom download URL for speed test
	DownloadSize    string        `json:"downloadSize"`             // endpoint preset key (see download.GetDownloadURL)
	Threads         int           `json:"threads"`                  // parallel download connections per node (1 = single thread)
	UploadEnable    bool          `json:"uploadEnable"`             // also test upload speed after download
	UploadURL       string        `json:"uploadUrl"`                // custom upload URL (POST sink); optional
	UploadSize      string        `json:"uploadSize"`               // upload endpoint preset key (see download.GetUploadURL)
	Appearance      string        `json:"appearance"`               // image appearance: "light"(default) | "dark"
	DownloadHeaders []HeaderKV    `json:"downloadHeaders"`          // 用户自定义下载请求头(可选)
	UploadHeaders   []HeaderKV    `json:"uploadHeaders"`            // 用户自定义上传请求头(可选)
}

type CMDOptions struct {
	Timeout       int
	Concurrency   int
	Output        string // json, text, pic, none
	OutputFile    string // output file path for JSON result
	OutputPicPath string // output pic path (can be used with any output mode)
	DownloadURL   string
	DownloadSize  string
	Threads       int    // parallel download connections per node
	Mode          string // pingonly, speedonly, all
	Silent        bool   // -log-level silent: 不打印 stderr 进度
}

type JSONOutput struct {
	Nodes        []render.Node      `json:"nodes"`
	Options      ProfileTestOptions `json:"options"`
	Traffic      int64              `json:"traffic"`
	Duration     string             `json:"duration"`
	SuccessCount int                `json:"successCount"`
	LinksCount   int                `json:"linksCount"`
}

// parseMessage 返回 links 及与之对齐的 groups(每链接组名,可空)。
// 单订阅路径 groups 为 nil(所有节点用 Options.GroupName)。
func parseMessage(message []byte) ([]string, []string, *ProfileTestOptions, error) {
	options := &ProfileTestOptions{}
	err := json.Unmarshal(message, options)
	if err != nil {
		return nil, nil, nil, err
	}
	options.Timeout = time.Duration(int(options.Timeout)) * time.Second
	if options.GroupName == "?empty?" || options.GroupName == "" {
		options.GroupName = "Default"
	}
	if options.Timeout < 8 {
		options.Timeout = 8
	}
	if options.Concurrency < 1 {
		options.Concurrency = 1
	}
	if options.Threads < 1 {
		options.Threads = 1
	}
	if options.Threads > 256 {
		options.Threads = 256
	}
	if options.UploadSize == "" {
		options.UploadSize = "cloudflare"
	}
	if options.TestMode == RETEST {
		return options.Links, nil, options, nil
	}
	options.TestMode = ALLTEST

	// 多订阅:逐条抓取/解析,拼出 links 及对齐的 groups;单条失败跳过不影响其它。
	if len(options.Subscriptions) > 0 {
		var links []string
		var groups []string
		for _, e := range options.Subscriptions {
			sub := strings.TrimSpace(e.Subscription)
			if sub == "" {
				continue
			}
			ls, err := ParseLinks(sub)
			if err != nil || len(ls) == 0 {
				slog.Debug("skip subscription entry", "group", e.Group, "err", err, "count", len(ls))
				continue
			}
			g := strings.TrimSpace(e.Group)
			for _, l := range ls {
				links = append(links, l)
				groups = append(groups, g)
			}
		}
		if len(links) == 0 {
			return nil, nil, nil, fmt.Errorf("no profile found")
		}
		return links, groups, options, nil
	}

	// 单订阅(兼容旧前端)
	links, err := ParseLinks(options.Subscription)
	if err != nil {
		return nil, nil, nil, err
	}
	return links, nil, options, nil
}

// groupFor 返回第 i 个链接的组名:优先用对齐的 Groups[i](非空),否则回退默认组名。
func (p *ProfileTest) groupFor(i int) string {
	if i >= 0 && i < len(p.Groups) && p.Groups[i] != "" {
		return p.Groups[i]
	}
	return p.Options.GroupName
}

type MessageWriter interface {
	WriteMessage(messageType int, data []byte) error
}

type OutputMessageWriter struct {
}

func (p *OutputMessageWriter) WriteMessage(messageType int, data []byte) error {
	slog.Debug("ws message", "data", string(data))
	return nil
}

type EmptyMessageWriter struct {
}

func (w *EmptyMessageWriter) WriteMessage(messageType int, data []byte) error {
	return nil
}

type ProfileTest struct {
	Writer      MessageWriter
	Options     *ProfileTestOptions
	MessageType int
	Links       []string
	Groups      []string // 与 Links 对齐的每链接组名(可空);nil 或空则该节点回退到 Options.GroupName
	mu          sync.Mutex
	wg          sync.WaitGroup // wait for all to finish

	// 测速机自身公网出口信息(全局一份,best-effort)。ipDone 在抓取协程结束时关闭。
	ipMu   sync.Mutex
	ipV4   *PublicIP
	ipV6   *PublicIP
	ipDone chan struct{}
}

// shouldFetchIP 仅在会产出图片时抓取(避免 json/text/none 模式引入外部请求)。
func (p *ProfileTest) shouldFetchIP() bool {
	if p.Options.OutputMode == PIC_BASE64 || p.Options.OutputMode == PIC_PATH {
		return true
	}
	return p.Options.OutputPicPath != ""
}

// startIPFetch 在测速开始时异步抓取公网 v4/v6+geo,就绪即发 "ipinfo" 消息。
func (p *ProfileTest) startIPFetch(ctx context.Context) {
	p.ipDone = make(chan struct{})
	go func() {
		defer close(p.ipDone)
		v4, v6 := FetchBoth(ctx)
		p.ipMu.Lock()
		p.ipV4, p.ipV6 = v4, v6
		p.ipMu.Unlock()
		if v4 != nil || v6 != nil {
			p.WriteMessage(getIPInfoMsg(v4, v6))
		}
	}()
}

// ipInfoLines 返回图片页脚的两行文本。抓取从测速开始就并行进行,通常渲染时已就绪;
// 这里最多再等 3s,避免个别很快结束的测试(pingonly/全部失败)被慢速 IP 源拖住出图。
// 超时则本次出图不带 IP(IP 仍会随 WS 的 ipinfo 消息在 GUI 展示)。
func (p *ProfileTest) ipInfoLines() (v4Line, v6Line string) {
	if p.ipDone != nil {
		select {
		case <-p.ipDone:
		case <-time.After(3 * time.Second):
		}
	}
	p.ipMu.Lock()
	defer p.ipMu.Unlock()
	return p.ipV4.Line("IPv4"), p.ipV6.Line("IPv6")
}

func (p *ProfileTest) WriteMessage(data []byte) error {
	var err error
	if p.Writer != nil {
		p.mu.Lock()
		err = p.Writer.WriteMessage(p.MessageType, data)
		p.mu.Unlock()
	}
	return err
}

func (p *ProfileTest) WriteString(data string) error {
	b := []byte(data)
	return p.WriteMessage(b)
}

// api
// render.Node contain the final test result
func (p *ProfileTest) TestAll(ctx context.Context, trafficChan chan<- int64) (chan render.Node, error) {
	links := p.Links
	linksCount := len(links)
	if linksCount < 1 {
		return nil, fmt.Errorf("profile not found")
	}
	nodeChan := make(chan render.Node, linksCount)
	go func(context.Context) {
		guard := make(chan int, p.Options.Concurrency)
		for i := range links {
			p.wg.Add(1)
			id := i
			link := links[i]
			select {
			case guard <- i:
				go func(id int, link string, c <-chan int, nodeChan chan<- render.Node) {
					p.testOne(ctx, id, link, nodeChan, trafficChan)
					<-c
				}(id, link, guard, nodeChan)
			case <-ctx.Done():
				return
			}
		}
		// p.wg.Wait()
		// if trafficChan != nil {
		// 	close(trafficChan)
		// }
	}(ctx)
	return nodeChan, nil
}

// run 只跑测速并收集汇总(started/gotservers/eof 消息、公网 IP 抓取、duration/traffic/成功计数),
// 不排序、不写盘/推图。WS(经 testAll)与 CLI(经 TestFromCMD)共用。返回未排序(按 Id)的 nodes。
func (p *ProfileTest) run(ctx context.Context) (render.Nodes, TestSummary, error) {
	linksCount := len(p.Links)
	if linksCount < 1 {
		p.WriteString(SPEEDTEST_ERROR_NONODES)
		return nil, TestSummary{}, fmt.Errorf("no profile found")
	}
	start := time.Now()
	p.WriteMessage(getMsgByte(-1, "started"))
	// 异步抓取测速机自身公网出口 IP+geo(仅在会出图片时);就绪即通过 WS 推送。
	if p.shouldFetchIP() {
		p.startIPFetch(ctx)
	}
	step := 9
	if linksCount > 200 {
		step = linksCount / 20
		if step > 50 {
			step = 50
		}
	}
	for i := 0; i < linksCount; {
		end := i + step
		if end > linksCount {
			end = linksCount
		}
		links := p.Links[i:end]
		groups := make([]string, len(links))
		for j := range links {
			groups[j] = p.groupFor(i + j)
		}
		msg := gotserversMsg(i, links, groups)
		p.WriteMessage(msg)
		i += step
	}
	guard := make(chan int, p.Options.Concurrency)
	nodeChan := make(chan render.Node, linksCount)

	nodes := make(render.Nodes, linksCount)
	for i := range p.Links {
		p.wg.Add(1)
		id := i
		link := ""
		if len(p.Options.TestIDs) > 0 && len(p.Options.Links) > 0 {
			id = p.Options.TestIDs[i]
			link = p.Options.Links[i]
		}
		select {
		case guard <- i:
			go func(id int, link string, c <-chan int, nodeChan chan<- render.Node) {
				p.testOne(ctx, id, link, nodeChan, nil)
				_ = p.WriteMessage(getMsgByte(id, "endone"))
				<-c
			}(id, link, guard, nodeChan)
		case <-ctx.Done():
			return nil, TestSummary{}, ctx.Err()
		}
	}
	p.wg.Wait()
	p.WriteMessage(getMsgByte(-1, "eof"))
	duration := FormatDuration(time.Since(start))
	successCount := 0
	var traffic int64 = 0
	for i := 0; i < linksCount; i++ {
		node := <-nodeChan
		node.Link = p.Links[node.Id]
		nodes[node.Id] = node
		traffic += node.Traffic
		if node.Success {
			successCount += 1
		}
	}
	close(nodeChan)

	summary := TestSummary{
		Nodes:        nodes,
		Traffic:      traffic,
		Duration:     duration,
		SuccessCount: successCount,
		LinksCount:   linksCount,
	}
	return nodes, summary, nil
}

// testAll 保持旧签名与副作用:run + 按 OutputMode 排序并写盘/推图(WS 与 HTTP 客户端用)。
func (p *ProfileTest) testAll(ctx context.Context) (render.Nodes, error) {
	nodes, summary, err := p.run(ctx)
	if err != nil {
		// 保持旧行为:被取消时返回 (nil, nil),不视为错误。
		if ctx.Err() != nil {
			return nil, nil
		}
		return nil, err
	}

	if p.Options.OutputMode == PIC_NONE {
		return nodes, nil
	}

	traffic, duration := summary.Traffic, summary.Duration
	successCount, linksCount := summary.SuccessCount, summary.LinksCount

	// sort nodes
	nodes.Sort(p.Options.SortMethod)
	// save json
	if p.Options.OutputMode == JSON_OUTPUT {
		p.saveJSON(nodes, traffic, duration, successCount, linksCount)
	} else if p.Options.OutputMode == TEXT_OUTPUT {
		p.saveText(nodes)
	} else {
		// render the result to pic
		p.renderPic(nodes, traffic, duration, successCount, linksCount)
	}

	// generate pic if OutputPicPath is set (can be used with any output mode)
	if p.Options.OutputPicPath != "" && p.Options.OutputMode != PIC_PATH && p.Options.OutputMode != PIC_BASE64 {
		p.savePic(nodes, traffic, duration, successCount, linksCount)
	}

	return nodes, nil
}

func (p *ProfileTest) renderPic(nodes render.Nodes, traffic int64, duration string, successCount int, linksCount int) error {
	fontPath := "WenQuanYiMicroHei-01.ttf"
	options := render.NewTableOptions(40, 30, 0.5, 0.5, p.Options.FontSize, 0.5, fontPath, p.Options.Language, p.Options.Theme, "Asia/Shanghai", FontBytes)
	options.SetAppearance(p.Options.Appearance)
	v4Line, v6Line := p.ipInfoLines()
	options.SetIPInfo(v4Line, v6Line)
	table, err := render.NewTableWithOption(nodes, &options)
	if err != nil {
		return err
	}
	// msg := fmt.Sprintf("Total Traffic : %s. Total Time : %s. Working Nodes: [%d/%d]", download.ByteCountIECTrim(traffic), duration, successCount, linksCount)
	msg := table.FormatTraffic(download.ByteCountIECTrim(traffic), duration, fmt.Sprintf("%d/%d", successCount, linksCount))
	if p.Options.OutputMode == PIC_PATH {
		table.Draw("out.png", msg)
		p.WriteMessage(getMsgByte(-1, "picdata", "out.png"))
		return nil
	}
	if picdata, err := table.EncodeB64(msg); err == nil {
		p.WriteMessage(getMsgByte(-1, "picdata", picdata))
	}
	return nil
}

func (p *ProfileTest) saveJSON(nodes render.Nodes, traffic int64, duration string, successCount int, linksCount int) error {
	jsonOutput := JSONOutput{
		Nodes:        nodes,
		Options:      *p.Options,
		Traffic:      traffic,
		Duration:     duration,
		SuccessCount: successCount,
		LinksCount:   linksCount,
	}
	data, err := json.MarshalIndent(&jsonOutput, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile("output.json", data, 0644)
}

func (p *ProfileTest) savePic(nodes render.Nodes, traffic int64, duration string, successCount int, linksCount int) error {
	fontPath := "WenQuanYiMicroHei-01.ttf"
	options := render.NewTableOptions(40, 30, 0.5, 0.5, p.Options.FontSize, 0.5, fontPath, p.Options.Language, p.Options.Theme, "Asia/Shanghai", FontBytes)
	options.SetAppearance(p.Options.Appearance)
	v4Line, v6Line := p.ipInfoLines()
	options.SetIPInfo(v4Line, v6Line)
	table, err := render.NewTableWithOption(nodes, &options)
	if err != nil {
		return err
	}
	msg := table.FormatTraffic(download.ByteCountIECTrim(traffic), duration, fmt.Sprintf("%d/%d", successCount, linksCount))
	table.Draw(p.Options.OutputPicPath, msg)
	slog.Info("pic result saved", "path", p.Options.OutputPicPath)
	return nil
}

func (p *ProfileTest) saveText(nodes render.Nodes) error {
	var links []string
	for _, node := range nodes {
		if node.Ping != "0" || node.AvgSpeed > 0 || node.MaxSpeed > 0 {
			links = append(links, node.Link)
		}
	}
	data := []byte(strings.Join(links, "\n"))
	return os.WriteFile("output.txt", data, 0644)
}

func (p *ProfileTest) testOne(ctx context.Context, index int, link string, nodeChan chan<- render.Node, trafficChan chan<- int64) error {
	// panic
	defer p.wg.Done()
	if link == "" {
		link = p.Links[index]
		link = strings.SplitN(link, "^", 2)[0]
	}
	cfg, err := config.Link2Config(link)
	if err != nil {
		return err
	}
	remarks := cfg.Remarks
	if err != nil || remarks == "" {
		remarks = fmt.Sprintf("Profile %d", index)
	}
	protocol := cfg.Protocol
	if (cfg.Protocol == "vmess" || cfg.Protocol == "trojan") && cfg.Net != "" {
		protocol = fmt.Sprintf("%s/%s", cfg.Protocol, cfg.Net)
	}
	elapse, err := p.pingLink(index, link)
	slog.Debug("ping result", "index", index, "remarks", remarks, "elapse_ms", elapse)
	if err != nil {
		node := render.Node{
			Id:       index,
			Group:    p.groupFor(index),
			Remarks:  remarks,
			Protocol: protocol,
			Ping:     fmt.Sprintf("%d", elapse),
			AvgSpeed: 0,
			MaxSpeed: 0,
			Success:  elapse > 0,
		}
		nodeChan <- node
		return err
	}
	p.WriteMessage(getMsgByte(index, "startspeed"))
	downloadURL := download.GetDownloadURL(p.Options.DownloadSize, p.Options.DownloadURL)
	dlHeaders := p.requestHeaders(p.Options.DownloadSize, true)
	dAvg, dMax, dSum := p.runSpeedPhase(ctx, index, remarks, "gotspeed", trafficChan,
		func(runCtx context.Context, ch chan<- int64, startCh chan<- time.Time) (int64, error) {
			return download.DownloadWithURLThreads(runCtx, link, p.Options.Timeout, p.Options.Timeout, ch, startCh, downloadURL, p.Options.Threads, dlHeaders)
		})

	// 上传阶段(可选,串行于下载之后;PingOnly 已在 pingLink 提前返回,不会到这里)。
	// ctx 已取消(用户终止/断开)时跳过,避免发出多余的 startupload 并空转建连。
	var uAvg, uMax int64
	if p.Options.UploadEnable && ctx.Err() == nil {
		p.WriteMessage(getMsgByte(index, "startupload"))
		uploadURL := download.GetUploadURL(p.Options.UploadSize, p.Options.UploadURL)
		upHeaders := p.requestHeaders(p.Options.UploadSize, false)
		uAvg, uMax, _ = p.runSpeedPhase(ctx, index, remarks, "gotupload", trafficChan,
			func(runCtx context.Context, ch chan<- int64, startCh chan<- time.Time) (int64, error) {
				return download.UploadWithURLThreads(runCtx, link, p.Options.Timeout, p.Options.Timeout, ch, startCh, uploadURL, p.Options.Threads, upHeaders)
			})
	}

	node := render.Node{
		Id:             index,
		Group:          p.groupFor(index),
		Remarks:        remarks,
		Protocol:       protocol,
		Ping:           fmt.Sprintf("%d", elapse),
		AvgSpeed:       dAvg,
		MaxSpeed:       dMax,
		UploadSpeed:    uAvg,
		MaxUploadSpeed: uMax,
		Success:        true,
		Traffic:        dSum,
	}
	nodeChan <- node
	return nil
}

// runSpeedPhase 消费一路每秒样本 channel,发对应实时消息(gotspeed/gotupload),
// 返回该方向的 avg/max/sum。run 负责把样本喂进 ch(DownloadWithURLThreads / UploadWithURLThreads)。
// 生命周期:run 返回后 close(ch),<-done 保证消费协程排空缓冲并结束;avg/max/sum 的读取
// happens-after <-done,无竞态、无早投 Node、无 goroutine 泄漏。
func (p *ProfileTest) runSpeedPhase(ctx context.Context, index int, remarks, msgType string, trafficChan chan<- int64,
	run func(ctx context.Context, ch chan<- int64, startCh chan<- time.Time) (int64, error)) (avg, max, sum int64) {
	ch := make(chan int64, 1)
	startCh := make(chan time.Time, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		start := time.Now()
		for {
			select {
			case speed, ok := <-ch:
				if !ok || speed < 0 {
					return
				}
				sum += speed
				// 平均速度用毫秒整数运算并对耗时取下限 1ms,避免 <1ms 内到达样本时
				// float64/0 得 +Inf、int64(+Inf)=MinInt64 的垃圾值。
				elapsedMs := time.Since(start).Milliseconds()
				if elapsedMs < 1 {
					elapsedMs = 1
				}
				avg = sum * 1000 / elapsedMs
				if max < speed {
					max = speed
				}
				slog.Debug("speed sample", "index", index, "remarks", remarks, "dir", msgType, "speed", download.ByteCountIEC(speed))
				p.WriteMessage(getMsgByte(index, msgType, avg, max, speed))
				if trafficChan != nil {
					select {
					case trafficChan <- speed:
					case <-ctx.Done():
					}
				}
			case s := <-startCh:
				start = s
			case <-ctx.Done():
				slog.Debug("speed test cancelled", "index", index, "dir", msgType)
				return
			}
		}
	}()
	speed, _ := run(ctx, ch, startCh)
	close(ch)
	<-done
	if speed < 1 {
		p.WriteMessage(getMsgByte(index, msgType, -1, -1, 0))
	}
	return avg, max, sum
}

// requestHeaders 组装下载/上传请求头:按方向取用户自定义头(下载用 DownloadHeaders,
// 上传用 UploadHeaders;前端已根据「统一」开关解析好两份)。自建 Worker 的 X-Speedtest-Key
// 现由用户在自定义请求头里自行填写(不再单独字段)。下载方向对 worker 端点额外带
// Accept-Encoding: identity 避免压缩影响吞吐计量。无任何头时返回 nil。
func (p *ProfileTest) requestHeaders(sizeKey string, isDownload bool) map[string]string {
	h := map[string]string{}
	src := p.Options.UploadHeaders
	if isDownload {
		src = p.Options.DownloadHeaders
	}
	for _, kv := range src {
		if name := strings.TrimSpace(kv.Name); name != "" {
			h[name] = kv.Value
		}
	}
	if sizeKey == "worker" && isDownload {
		h["Accept-Encoding"] = "identity"
	}
	if len(h) == 0 {
		return nil
	}
	return h
}

func (p *ProfileTest) pingLink(index int, link string) (int64, error) {
	if p.Options.SpeedTestMode == SpeedOnly {
		return 0, nil
	}
	if link == "" {
		link = p.Links[index]
	}
	p.WriteMessage(getMsgByte(index, "startping"))
	elapse, err := request.PingLink(link, 2)
	p.WriteMessage(getMsgByte(index, "gotping", elapse))
	if elapse < 1 {
		p.WriteMessage(getMsgByte(index, "gotspeed", -1, -1, 0))
		return 0, err
	}
	if p.Options.SpeedTestMode == PingOnly {
		p.WriteMessage(getMsgByte(index, "gotspeed", -1, -1, 0))
		return elapse, errors.New(PingOnly)
	}
	return elapse, err
}

func FormatDuration(duration time.Duration) string {
	h := duration / time.Hour
	duration -= h * time.Hour
	m := duration / time.Minute
	duration -= m * time.Minute
	s := duration / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	return fmt.Sprintf("%dm %ds", m, s)
}

func isYamlFile(filePath string) bool {
	return strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml")
}

// api
func PeekClash(input string, n int) ([]string, error) {
	scanner := bufio.NewScanner(strings.NewReader(input))
	proxiesStart := false
	data := []byte{}
	linkCount := 0
	for scanner.Scan() {
		b := scanner.Bytes()
		trimLine := strings.TrimSpace(string(b))
		if trimLine == "proxy-groups:" || trimLine == "rules:" || trimLine == "Proxy Group:" {
			break
		}
		if proxiesStart {
			if _, err := config.ParseBaseProxy(trimLine); err != nil {
				continue
			}
			if strings.HasPrefix(trimLine, "-") {
				if linkCount >= n {
					break
				}
				linkCount += 1
			}
			data = append(data, b...)
			data = append(data, byte('\n'))
			continue
		}
		if !proxiesStart && (trimLine == "proxies:" || trimLine == "Proxy:") {
			proxiesStart = true
			b = []byte("proxies:")
		}
		data = append(data, b...)
		data = append(data, byte('\n'))
	}
	// fmt.Println(string(data))
	links, err := parseClashByte(data)
	if err != nil || len(links) < 1 {
		return []string{}, err
	}
	endIndex := n
	if endIndex > len(links) {
		endIndex = len(links)
	}
	return links[:endIndex], nil
}
