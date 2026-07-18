package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	grpcServer "github.com/1orz/proxy-speedtest/api/rpc/liteserver"
	C "github.com/1orz/proxy-speedtest/constant"
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
	output       = flag.String("output", "json", "output format: json, text, pic, none")
	outputFile   = flag.String("output-file", "", "output file path for JSON result")
	outputPic    = flag.String("output-pic", "", "output pic path (can be used with any output format)")
	downloadURL  = flag.String("download-url", "", "custom download URL for speed test")
	downloadSize = flag.String("download-size", "", "download size: 10mb, 100mb, or custom URL bytes param")
	mode         = flag.String("mode", "all", "test mode: pingonly, speedonly, all")
)

func main() {
	flag.Parse()
	if *version {
		fmt.Printf("LiteSpeedTest  %s %s %s with %s %s\n", C.Version, runtime.GOOS, runtime.GOARCH, runtime.Version(), C.BuildTime)
		return
	}
	link := ""
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if _, err := utils.CheckLink(arg); err == nil {
			link = arg
			break
		}
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
			Mode:          *mode,
		}
		if err := webServer.TestFromCMD(*test, conf, cmdOpts); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Start gRPC server
	if *grpc {
		if err := grpcServer.StartServer(uint16(*port), *bind); err != nil {
			log.Fatalln(err)
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
			Mode:          *mode,
		}
		if err := webServer.TestFromCMD(link, conf, cmdOpts); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Start web server
	if len(os.Args) < 2 {
		*port = 10888
	}
	if err := webServer.ServeFile(*port, *bind); err != nil {
		log.Fatalln(err)
	}
}
