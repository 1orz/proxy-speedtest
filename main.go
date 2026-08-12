package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	grpcServer "github.com/1orz/proxy-speedtest/api/rpc/liteserver"
	"github.com/1orz/proxy-speedtest/config"
	C "github.com/1orz/proxy-speedtest/constant"
	"github.com/1orz/proxy-speedtest/log"
	"github.com/1orz/proxy-speedtest/utils"
	webServer "github.com/1orz/proxy-speedtest/web"

	// Register xray protocols
	_ "github.com/1orz/proxy-speedtest/internal/xray"
)

// headerFlags implements a repeatable curl-style HTTP header flag.
type headerFlags []webServer.HeaderKV

func (h *headerFlags) String() string {
	values := make([]string, 0, len(*h))
	for _, header := range *h {
		values = append(values, header.Name+": "+header.Value)
	}
	return strings.Join(values, ", ")
}

func (h *headerFlags) Set(value string) error {
	name, headerValue, ok := strings.Cut(value, ":")
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		return fmt.Errorf("header must use 'Name: Value' format")
	}
	*h = append(*h, webServer.HeaderKV{Name: name, Value: strings.TrimSpace(headerValue)})
	return nil
}

// optionalBool records whether a boolean flag was explicitly provided, so a
// config-file value can be preserved when the CLI flag is absent.
type optionalBool struct {
	value bool
	set   bool
}

func (b *optionalBool) String() string   { return strconv.FormatBool(b.value) }
func (b *optionalBool) IsBoolFlag() bool { return true }
func (b *optionalBool) Set(value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	b.value = parsed
	b.set = true
	return nil
}

var (
	port            = flag.Int("p", 8090, "set port")
	bind            = flag.String("bind", "", "bind web server to an address or interface name, e.g. 100.x.x.x or tailscale0 (empty = all interfaces)")
	test            = flag.String("test", "", "test from command line with subscription link or file")
	direct          = flag.Bool("direct", false, "direct speed test without any proxy: no subscription needed, measures this machine's own download/upload speed")
	conf            = flag.String("config", "", "config file path (JSON format)")
	grpc            = flag.Bool("grpc", false, "start grpc server")
	version         = flag.Bool("v", false, "show current version")
	timeout         = flag.Int("timeout", 16, "timeout for each node test in seconds")
	concurrency     = flag.Int("concurrency", 2, "number of concurrent tests")
	output          = flag.String("output", "json", "output formats (comma-separated): json, csv, text, table, pic, none")
	outputFile      = flag.String("output-file", "", "output file path for JSON result")
	outputPic       = flag.String("output-pic", "", "output pic path (can be used with any output format)")
	downloadURL     = flag.String("download-url", "", "custom download URL for speed test")
	downloadSize    = flag.String("download-size", "", "download endpoint preset key (cloudflare, hetzner-de, hetzner-us, linode-jp, vultr-sg, ovh-eu, datapacket-us, huawei-cn, worker)")
	upload          optionalBool
	uploadURL       = flag.String("upload-url", "", "custom upload URL (HTTP POST sink) for speed test")
	uploadSize      = flag.String("upload-size", "", "upload endpoint preset key (cloudflare, dlptest, worker, cdn); cdn = self-hosted object-storage gateway (PUT), pass its auth via -uH \"x-upload-key: ...\"")
	allHeaders      headerFlags
	downloadHeaders headerFlags
	uploadHeaders   headerFlags
	threads         = flag.Int("threads", 1, "parallel download connections per node speed test (1=single thread)")
	mode            = flag.String("mode", "all", "test mode: pingonly, speedonly, all")
	logLevel        = flag.String("log-level", "info", "log level: debug, info, warning, error, silent")
)

// init 注册短参数别名,绑定到与长参数相同的变量(后解析者生效)。
func init() {
	flag.StringVar(output, "o", "json", "alias for -output")
	flag.StringVar(outputFile, "f", "", "alias for -output-file")
	flag.Var(&upload, "upload", "also test upload speed after download")
	flag.Var(&upload, "upload-enable", "alias for -upload")
	flag.Var(&allHeaders, "H", "HTTP header for both download and upload; repeatable, format: 'Name: Value'")
	flag.Var(&downloadHeaders, "dH", "download-only HTTP header; repeatable, format: 'Name: Value'")
	flag.Var(&uploadHeaders, "uH", "upload-only HTTP header; repeatable, format: 'Name: Value'")
}

func commandOptions() *webServer.CMDOptions {
	return &webServer.CMDOptions{
		Timeout:         *timeout,
		Concurrency:     *concurrency,
		Output:          *output,
		OutputFile:      *outputFile,
		OutputPicPath:   *outputPic,
		DownloadURL:     *downloadURL,
		DownloadSize:    *downloadSize,
		UploadEnable:    upload.value,
		UploadEnableSet: upload.set,
		UploadURL:       *uploadURL,
		UploadSize:      *uploadSize,
		CommonHeaders:   []webServer.HeaderKV(allHeaders),
		DownloadHeaders: []webServer.HeaderKV(downloadHeaders),
		UploadHeaders:   []webServer.HeaderKV(uploadHeaders),
		Threads:         *threads,
		Mode:            *mode,
		Silent:          *logLevel == "silent",
	}
}

// fatal 无条件把致命错误打到 stderr 再退出;不经日志级别门控,
// 保证即便 -log-level silent 也能看到进程为何失败。
func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "fatal: %s: %v\n", msg, err)
	os.Exit(1)
}

func main() {
	flag.Parse()
	// 位置参数里的分享链接(vmess:// 等)→ 单链接直测。用 flag.Args() 而不是扫描 os.Args:
	//   1) 避免把 --download-url 等 flag 的取值(如 http://...)误判成链接;
	//   2) Go 的 flag 在首个位置参数处停止解析,这里把链接之后出现的 token 再解析一遍,
	//      使 `proxy-speedtest "vmess://..." -o csv` 这种「flag 在链接后」也能生效。
	link := ""
	if flag.NArg() > 0 {
		if _, err := utils.CheckLink(flag.Arg(0)); err == nil {
			link = flag.Arg(0)
			if flag.NArg() > 1 {
				_ = flag.CommandLine.Parse(flag.Args()[1:])
			}
		}
	}
	log.Setup(*logLevel)
	if *version {
		fmt.Printf("LiteSpeedTest  %s %s %s with %s %s\n", C.Version, runtime.GOOS, runtime.GOARCH, runtime.Version(), C.BuildTime)
		return
	}

	// Test from command line
	if *test != "" {
		cmdOpts := commandOptions()
		if err := webServer.TestFromCMD(*test, conf, cmdOpts); err != nil {
			fatal("command-line test failed", err)
		}
		return
	}

	// 直连测速:不需要订阅/节点,合成一条 direct:// 走同一条 CLI 流水线,
	// 只测本机的下载(必填,默认 Cloudflare 端点)与可选的上传。
	// 已经给了 -test 或位置参数链接时以后者为准,-direct 忽略。
	if *direct && *test == "" && link == "" {
		cmdOpts := commandOptions()
		if err := webServer.TestFromCMD(config.DirectLink, conf, cmdOpts); err != nil {
			fatal("direct test failed", err)
		}
		return
	}

	// Start gRPC server
	if *grpc {
		if err := grpcServer.StartServer(uint16(*port), *bind); err != nil {
			fatal("grpc server failed", err)
		}
		return
	}

	// Test a single link directly (if provided as argument)
	if link != "" {
		cmdOpts := commandOptions()
		if err := webServer.TestFromCMD(link, conf, cmdOpts); err != nil {
			fatal("single-link test failed", err)
		}
		return
	}

	// Start web server
	if len(os.Args) < 2 {
		*port = 10888
	}
	if err := webServer.ServeFile(*port, *bind); err != nil {
		fatal("web server failed", err)
	}
}
