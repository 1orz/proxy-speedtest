package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	grpcServer "github.com/1orz/proxy-speedtest/api/rpc/liteserver"
	C "github.com/1orz/proxy-speedtest/constant"
	"github.com/1orz/proxy-speedtest/log"
	"github.com/1orz/proxy-speedtest/utils"
	webServer "github.com/1orz/proxy-speedtest/web"

	// Register xray protocols
	_ "github.com/1orz/proxy-speedtest/internal/xray"
)

var (
	port         = flag.Int("p", 8090, "set port")
	bind         = flag.String("bind", "", "bind web server to an address or interface name, e.g. 100.x.x.x or tailscale0 (empty = all interfaces)")
	test         = flag.String("test", "", "test from command line with subscription link or file")
	conf         = flag.String("config", "", "config file path (JSON format)")
	grpc         = flag.Bool("grpc", false, "start grpc server")
	version      = flag.Bool("v", false, "show current version")
	timeout      = flag.Int("timeout", 16, "timeout for each node test in seconds")
	concurrency  = flag.Int("concurrency", 2, "number of concurrent tests")
	output       = flag.String("output", "json", "output formats (comma-separated): json, csv, text, table, pic, none")
	outputFile   = flag.String("output-file", "", "output file path for JSON result")
	outputPic    = flag.String("output-pic", "", "output pic path (can be used with any output format)")
	downloadURL  = flag.String("download-url", "", "custom download URL for speed test")
	downloadSize = flag.String("download-size", "", "download endpoint preset key (cloudflare, hetzner-de, hetzner-us, linode-jp, vultr-sg, ovh-eu, datapacket-us, huawei-cn)")
	threads      = flag.Int("threads", 1, "parallel download connections per node speed test (1=single thread)")
	mode         = flag.String("mode", "all", "test mode: pingonly, speedonly, all")
	logLevel     = flag.String("log-level", "info", "log level: debug, info, warning, error, silent")
)

// init 注册短参数别名,绑定到与长参数相同的变量(后解析者生效)。
func init() {
	flag.StringVar(output, "o", "json", "alias for -output")
	flag.StringVar(outputFile, "f", "", "alias for -output-file")
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
		cmdOpts := &webServer.CMDOptions{
			Timeout:       *timeout,
			Concurrency:   *concurrency,
			Output:        *output,
			OutputFile:    *outputFile,
			OutputPicPath: *outputPic,
			DownloadURL:   *downloadURL,
			DownloadSize:  *downloadSize,
			Threads:       *threads,
			Mode:          *mode,
			Silent:        *logLevel == "silent",
		}
		if err := webServer.TestFromCMD(*test, conf, cmdOpts); err != nil {
			fatal("command-line test failed", err)
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
		cmdOpts := &webServer.CMDOptions{
			Timeout:       *timeout,
			Concurrency:   *concurrency,
			Output:        *output,
			OutputFile:    *outputFile,
			OutputPicPath: *outputPic,
			DownloadURL:   *downloadURL,
			DownloadSize:  *downloadSize,
			Threads:       *threads,
			Mode:          *mode,
			Silent:        *logLevel == "silent",
		}
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
