package web

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/1orz/proxy-speedtest/config"
	"github.com/1orz/proxy-speedtest/download"
	"github.com/1orz/proxy-speedtest/utils"
	"github.com/1orz/proxy-speedtest/web/render"
)

var upgrader = websocket.Upgrader{}

func ServeFile(port int, bind string) error {
	// TODO: Mobile UI
	mux := http.NewServeMux()
	mux.HandleFunc("/", serverFile)
	mux.HandleFunc("/test", updateTest)
	mux.HandleFunc("/renderImage", renderImage)
	mux.HandleFunc("/getSubscriptionLink", getSubscriptionLink)
	mux.HandleFunc("/getSubscription", getSubscription)

	host, err := ResolveBindAddress(bind)
	if err != nil {
		return err
	}
	// host == "" listens on all interfaces (":port"), preserving previous behaviour
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	if bind == "" {
		slog.Info("server started", "url", fmt.Sprintf("http://127.0.0.1:%d", port))
		if ipAddr, err := localIP(); err == nil {
			slog.Info("server started", "url", fmt.Sprintf("http://%s", net.JoinHostPort(ipAddr.String(), strconv.Itoa(port))))
		}
	} else {
		slog.Info("server started", "url", fmt.Sprintf("http://%s", addr), "bind", bind)
	}
	return http.ListenAndServe(addr, accessLog(mux))
}

// ResolveBindAddress turns a user-supplied bind value into a listen host.
// Empty string -> all interfaces. A literal IP is used as-is. Anything else is
// treated as a network interface name and resolved to its first usable IP,
// which is handy for pinning the server to e.g. the tailscale0 interface.
func ResolveBindAddress(bind string) (string, error) {
	if bind == "" {
		return "", nil
	}
	if net.ParseIP(bind) != nil {
		return bind, nil
	}
	iface, err := net.InterfaceByName(bind)
	if err != nil {
		return "", fmt.Errorf("bind: interface %q not found: %w", bind, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("bind: cannot read addresses of %q: %w", bind, err)
	}
	var v6 string
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLinkLocalUnicast() {
			continue
		}
		if ip4 := ip.To4(); ip4 != nil {
			return ip4.String(), nil
		}
		if v6 == "" {
			v6 = ip.String()
		}
	}
	if v6 != "" {
		return v6, nil
	}
	return "", fmt.Errorf("bind: interface %q has no usable IP address", bind)
}

func serverFile(w http.ResponseWriter, r *http.Request) {
	h := http.FileServer(http.FS(guiStatic))
	r.URL.Path = "gui/dist" + r.URL.Path
	h.ServeHTTP(w, r)
}

func updateTest(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Debug("websocket upgrade failed", "err", err)
		return
	}
	defer c.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			slog.Debug("websocket read ended", "err", err)
			break
		}
		// log.Printf("recv: %s", message)
		links, options, err := parseMessage(message)
		if err != nil {
			msg := `{"info": "error", "reason": "invalidsub"}`
			c.WriteMessage(mt, []byte(msg))
			continue
		}
		if options.Unique {
			uniqueLinks := []string{}
			uniqueMap := map[string]struct{}{}
			for _, link := range links {
				cfg, err := config.Link2Config(link)
				if err != nil {
					continue
				}
				key := fmt.Sprintf("%s%d%s%s%s", cfg.Server, cfg.Port, cfg.Password, cfg.Protocol, cfg.SNI)
				if _, ok := uniqueMap[key]; !ok {
					uniqueLinks = append(uniqueLinks, link)
					uniqueMap[key] = struct{}{}
				}
			}
			links = uniqueLinks
		}
		p := ProfileTest{
			Writer:      c,
			MessageType: mt,
			Links:       links,
			Options:     options,
		}
		go p.testAll(ctx)
		// err = c.WriteMessage(mt, getMsgByte(0, "gotspeed"))
		// if err != nil {
		// 	log.Println("write:", err)
		// 	break
		// }
	}
}

// renderImageRequest 是"重新生成图片"端点的请求体:携带当前节点结果 + 渲染选项
// (语言/主题/appearance/字体/公网 IP),由前端用当前偏好组装。节点按传入顺序渲染(不再排序)。
type renderImageRequest struct {
	Language   string `json:"language"`
	Appearance string `json:"appearance"`
	Theme      string `json:"theme"`
	FontSize   int    `json:"fontSize"`
	Traffic    int64  `json:"traffic"`
	Duration   string `json:"duration"`
	Success    int    `json:"successCount"`
	Total      int    `json:"linksCount"`
	IPv4       string `json:"ipv4"`
	IPv6       string `json:"ipv6"`
	IPv4Geo    string `json:"ipv4geo"`
	IPv6Geo    string `json:"ipv6geo"`
	Nodes      []struct {
		Group       string `json:"group"`
		Remarks     string `json:"remarks"`
		Protocol    string `json:"protocol"`
		Ping        string `json:"ping"`
		AvgSpeed    int64  `json:"avgSpeed"`
		UploadSpeed int64  `json:"uploadSpeed"`
	} `json:"nodes"`
}

// renderImage 无状态地按当前偏好重渲染结果图片,返回 base64 PNG(data URL)。
// 用于用户测速后切换了语言/主题/深浅色,想不重测就更新图片。
func renderImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Body == nil {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req renderImageRequest
	if err := json.Unmarshal(data, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Nodes) == 0 {
		http.Error(w, "no nodes", http.StatusBadRequest)
		return
	}
	if req.FontSize < 8 {
		req.FontSize = 24
	}
	nodes := make(render.Nodes, 0, len(req.Nodes))
	for _, n := range req.Nodes {
		nodes = append(nodes, render.Node{
			Group:       n.Group,
			Remarks:     n.Remarks,
			Protocol:    n.Protocol,
			Ping:        n.Ping,
			AvgSpeed:    n.AvgSpeed,
			UploadSpeed: n.UploadSpeed,
		})
	}
	fontPath := "WenQuanYiMicroHei-01.ttf"
	options := render.NewTableOptions(40, 30, 0.5, 0.5, req.FontSize, 0.5, fontPath, req.Language, req.Theme, "Asia/Shanghai", FontBytes)
	options.SetAppearance(req.Appearance)
	options.SetIPInfo(ipDisplayLine("IPv4", req.IPv4, req.IPv4Geo), ipDisplayLine("IPv6", req.IPv6, req.IPv6Geo))
	table, err := render.NewTableWithOption(nodes, &options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	msg := table.FormatTraffic(download.ByteCountIECTrim(req.Traffic), req.Duration, fmt.Sprintf("%d/%d", req.Success, req.Total))
	b64, err := table.EncodeB64(msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"data": b64})
}

func readConfig(configPath string) (*ProfileTestOptions, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	options := &ProfileTestOptions{}
	if err = json.Unmarshal(data, options); err != nil {
		return nil, err
	}
	if options.Concurrency < 1 {
		options.Concurrency = 1
	}
	if options.Language == "" {
		options.Language = "en"
	}
	if options.Theme == "" {
		options.Theme = "rainbow"
	}
	if options.Appearance == "" {
		options.Appearance = "light"
	}
	if options.Timeout < 8 {
		options.Timeout = 8
	}
	options.Timeout = options.Timeout * time.Second
	return options, nil
}

func TestFromCMD(subscription string, configPath *string, cmdOpts *CMDOptions) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	options := ProfileTestOptions{
		GroupName:       "Default",
		SpeedTestMode:   "all",
		SortMethod:      "rspeed",
		Concurrency:     2,
		TestMode:        2,
		Subscription:    subscription,
		Language:        "en",
		FontSize:        24,
		Theme:           "rainbow",
		Appearance:      "light",
		Timeout:         15 * time.Second,
		GeneratePicMode: PIC_PATH,
		OutputMode:      PIC_PATH,
	}
	if configPath != nil && *configPath != "" {
		if opt, err := readConfig(*configPath); err == nil {
			options = *opt
			if options.GeneratePicMode != 0 {
				options.OutputMode = options.GeneratePicMode
			}
		}
	}
	// apply command line options (override config file)
	if cmdOpts != nil {
		if cmdOpts.Timeout > 0 {
			options.Timeout = time.Duration(cmdOpts.Timeout) * time.Second
		}
		if cmdOpts.Concurrency > 0 {
			options.Concurrency = cmdOpts.Concurrency
		}
		if cmdOpts.DownloadURL != "" {
			options.DownloadURL = cmdOpts.DownloadURL
		}
		if cmdOpts.DownloadSize != "" {
			options.DownloadSize = cmdOpts.DownloadSize
		}
		if cmdOpts.Threads > 0 {
			options.Threads = cmdOpts.Threads
			if options.Threads > 256 {
				options.Threads = 256
			}
		}
		// test mode: pingonly, speedonly, all
		switch cmdOpts.Mode {
		case "pingonly":
			options.SpeedTestMode = PingOnly
		case "speedonly":
			options.SpeedTestMode = SpeedOnly
		default:
			options.SpeedTestMode = "all"
		}
		switch cmdOpts.Output {
		case "json":
			options.OutputMode = JSON_OUTPUT
		case "text":
			options.OutputMode = TEXT_OUTPUT
		case "pic":
			options.OutputMode = PIC_PATH
		case "none":
			options.OutputMode = PIC_NONE
		}
		// output file path
		if cmdOpts.OutputFile != "" {
			options.OutputFilePath = cmdOpts.OutputFile
		}
		// output pic path (can be used with any output mode)
		if cmdOpts.OutputPicPath != "" {
			options.OutputPicPath = cmdOpts.OutputPicPath
		}
	}
	// check url
	if len(subscription) > 0 && subscription != options.Subscription {
		if _, err := url.Parse(subscription); err == nil {
			options.Subscription = subscription
		} else if _, err := os.Stat(subscription); err == nil {
			options.Subscription = subscription
		}
	}
	if jsonOpt, err := json.Marshal(options); err == nil {
		slog.Debug("cmd options", "json", string(jsonOpt))
	}
	nodes, err := TestContext(ctx, options, &OutputMessageWriter{})
	if err != nil {
		return err
	}
	// output JSON to stdout when output mode is json
	if options.OutputMode == JSON_OUTPUT {
		outputJSON(nodes, options)
	}
	return nil
}

func outputJSON(nodes render.Nodes, options ProfileTestOptions) {
	var traffic int64
	successCount := 0
	for _, node := range nodes {
		traffic += node.Traffic
		if node.Success {
			successCount++
		}
	}
	jsonOutput := JSONOutput{
		Nodes:        nodes,
		Options:      options,
		Traffic:      traffic,
		Duration:     "",
		SuccessCount: successCount,
		LinksCount:   len(nodes),
	}
	data, err := json.MarshalIndent(&jsonOutput, "", "  ")
	if err != nil {
		slog.Error("json marshal failed", "err", err)
		return
	}
	// output to file if OutputFilePath is set
	if options.OutputFilePath != "" {
		if err := os.WriteFile(options.OutputFilePath, data, 0644); err != nil {
			slog.Error("failed to write JSON file", "path", options.OutputFilePath, "err", err)
		} else {
			slog.Info("JSON result saved", "path", options.OutputFilePath)
		}
	} else {
		fmt.Println(string(data))
	}
}

// use as golang api
func TestContext(ctx context.Context, options ProfileTestOptions, w MessageWriter) (render.Nodes, error) {
	links, err := ParseLinks(options.Subscription)
	if err != nil {
		return nil, err
	}
	// outputMessageWriter := OutputMessageWriter{}
	p := ProfileTest{
		Writer:      w,
		MessageType: 1,
		Links:       links,
		Options:     &options,
	}
	return p.testAll(ctx)
}

// use as golang api
func TestAsyncContext(ctx context.Context, options ProfileTestOptions) (chan render.Node, []string, error) {
	links, err := ParseLinks(options.Subscription)
	if err != nil {
		return nil, nil, err
	}
	// outputMessageWriter := OutputMessageWriter{}
	p := ProfileTest{
		Writer:      nil,
		MessageType: ALLTEST,
		Links:       links,
		Options:     &options,
	}
	nodeChan, err := p.TestAll(ctx, nil)
	return nodeChan, links, err
}

func isPrivateIP(ip net.IP) bool {
	var privateIPBlocks []*net.IPNet
	for _, cidr := range []string{
		// don't check loopback ips
		//"127.0.0.0/8",    // IPv4 loopback
		//"::1/128",        // IPv6 loopback
		//"fe80::/10",      // IPv6 link-local
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
	} {
		_, block, _ := net.ParseCIDR(cidr)
		privateIPBlocks = append(privateIPBlocks, block)
	}

	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}

	return false
}

func localIP() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if isPrivateIP(ip) {
				return ip, nil
			}
		}
	}

	return nil, errors.New("no IP")
}

type GetSubscriptionLink struct {
	FilePath string `json:"filePath"`
	Group    string `json:"group"`
}

// subscriptionLinkMap stores mapping of hash -> file path for subscriptions
// Uses sync.Map for concurrent safety
var subscriptionLinkMap sync.Map

func getSubscriptionLink(w http.ResponseWriter, r *http.Request) {
	body := GetSubscriptionLink{}
	if r.Body == nil {
		http.Error(w, "Invalid Parameter", 400)
		return
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid Parameter", 400)
		return
	}
	if err = json.Unmarshal(data, &body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if len(body.FilePath) == 0 || len(body.Group) == 0 {
		http.Error(w, "Invalid Parameter", 400)
		return
	}
	ipAddr, err := localIP()
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	md5Hash := fmt.Sprintf("%x", md5.Sum([]byte(body.FilePath)))
	subscriptionLinkMap.Store(md5Hash, body.FilePath)
	subscriptionLink := fmt.Sprintf("http://%s:10888/getSubscription?key=%s&group=%s", ipAddr.String(), md5Hash, body.Group)
	fmt.Fprint(w, subscriptionLink)
}

// POST
func getSubscription(w http.ResponseWriter, r *http.Request) {
	queries := r.URL.Query()
	key := queries.Get("key")
	if len(key) < 1 {
		http.Error(w, "Key not found", 400)
		return
	}
	// sub format
	sub := queries.Get("sub")
	filePathValue, ok := subscriptionLinkMap.Load(key)
	if !ok {
		http.Error(w, "Wrong key", 400)
		return
	}
	filePath := filePathValue.(string)
	// convert yaml link
	if isYamlFile(filePath) && utils.IsUrl(filePath) {
		links, err := getSubscriptionLinks(filePath)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		b64Data := base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n")))
		w.Write([]byte(b64Data))
		return
	}
	// FIXME
	if isYamlFile(filePath) {
		data, err := writeClash(filePath)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.Write(data)
		return
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	if len(data) > 128 && strings.Contains(string(data[:128]), "proxies:") {
		if dataClash, err := writeClash(filePath); err == nil && len(dataClash) > 0 {
			data = dataClash
		}
	}
	// convert shadowrocket to v2ray
	if sub == "v2ray" {
		if dataShadowrocket, err := writeShadowrocket(data); err == nil && len(dataShadowrocket) > 0 {
			data = dataShadowrocket
		}
	}

	w.Write(data)
}

func writeClash(filePath string) ([]byte, error) {
	links, err := parseClashFileByLine(filePath)
	if err != nil {
		//
		return nil, err
	}
	subscription := []byte(strings.Join(links, "\n"))
	data := make([]byte, base64.StdEncoding.EncodedLen(len(subscription)))
	base64.StdEncoding.Encode(data, subscription)
	return data, nil
}

func writeShadowrocket(data []byte) ([]byte, error) {
	links, err := ParseLinks(string(data))
	if err != nil {
		return nil, err
	}
	newLinks := make([]string, 0, len(links))
	for _, link := range links {
		if strings.HasPrefix(link, "vmess://") && strings.Contains(link, "&") {
			if newLink, err := config.ShadowrocketLinkToVmessLink(link); err == nil {
				newLinks = append(newLinks, newLink)
			}
		} else {
			newLinks = append(newLinks, link)
		}
	}
	subscription := []byte(strings.Join(newLinks, "\n"))
	data = make([]byte, base64.StdEncoding.EncodedLen(len(subscription)))
	base64.StdEncoding.Encode(data, subscription)
	return data, nil
}
