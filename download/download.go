package download

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/1orz/proxy-speedtest/common/pool"
	"github.com/1orz/proxy-speedtest/internal/parser"
	"github.com/1orz/proxy-speedtest/internal/xray"
	"github.com/1orz/proxy-speedtest/stats"
)

const (
	// 微软 CDN 上的 dotnetfx35.exe(~231MB),仅作最终兜底,已验证可用。
	DownloadLinkDefault = "https://download.microsoft.com/download/2/0/E/20E90413-712F-438C-988E-FDAA79A8AC3D/dotnetfx35.exe"
	// Cloudflare 全球 Anycast。注意:__down?bytes= 的上限当前为 <1e8(>=1e8 直接 403),
	// 故取 10MB;文件偏小,建议配合多线程下载充分利用测试时长。
	CloudflareLink   = "https://speed.cloudflare.com/__down?bytes=10000000"
	HetznerDE1G      = "https://fsn1-speed.hetzner.com/1GB.bin"
	HetznerUS1G      = "https://ash-speed.hetzner.com/1GB.bin"
	LinodeJP100M     = "https://speedtest.tokyo2.linode.com/100MB-tokyo2.bin"
	VultrSG100M      = "https://sgp-ping.vultr.com/vultr.com.100MB.bin"
	OVH1G            = "https://proof.ovh.net/files/1Gb.dat"
	DataPacketUS100M = "http://lax.download.datapacket.com/100mb.bin"
	// 华为云镜像(~2.3GB Ubuntu ISO),国内有 CDN 节点、海外亦可达。
	HuaweiCN2G = "https://mirrors.huaweicloud.com/ubuntu-releases/bionic/ubuntu-18.04.6-desktop-amd64.iso"
)

// GetDownloadURL 把前端/命令行的端点 key 映射到具体下载 URL。key 需与前端
// DOWNLOAD_ENDPOINTS 保持一致;customURL 非空时优先使用。
func GetDownloadURL(size string, customURL string) string {
	if customURL != "" {
		return customURL
	}
	switch size {
	case "cloudflare":
		return CloudflareLink
	case "hetzner-de":
		return HetznerDE1G
	case "hetzner-us":
		return HetznerUS1G
	case "linode-jp":
		return LinodeJP100M
	case "vultr-sg":
		return VultrSG100M
	case "ovh-eu":
		return OVH1G
	case "datapacket-us":
		return DataPacketUS100M
	case "huawei-cn":
		return HuaweiCN2G
	default:
		return CloudflareLink
	}
}

type DownloadOption struct {
	URL              string
	DownloadTimeout  time.Duration
	HandshakeTimeout time.Duration
	Ranges           []Range
}

type Discard struct {
	total stats.Counter
}

func (e *Discard) Write(p []byte) (n int, err error) {
	n = len(p)
	pool.Put(p)
	e.total.Add(int64(n))
	return n, nil
}

func (e *Discard) Size() int64 {
	return e.total.Set(0)
}

func ByteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B/s", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB/s",
		float64(b)/float64(div), "KMGTPE"[exp])
}

func ByteCountIECTrim(b int64) string {
	result := ByteCountIEC(b)
	return strings.TrimSuffix(result, "/s")
}

// createDialer creates an xray dialer from a proxy link
func createDialer(link string) (*xray.Dialer, error) {
	config, err := parser.ParseLink(link)
	if err != nil {
		return nil, err
	}
	return xray.NewDialer(config)
}

func Download(link string, timeout time.Duration, handshakeTimeout time.Duration, resultChan chan<- int64, startChan chan<- time.Time) (int64, error) {
	return DownloadWithURL(link, timeout, handshakeTimeout, resultChan, startChan, DownloadLinkDefault)
}

func DownloadWithURL(link string, timeout time.Duration, handshakeTimeout time.Duration, resultChan chan<- int64, startChan chan<- time.Time, downloadURL string) (int64, error) {
	ctx := context.Background()
	dialer, err := createDialer(link)
	if err != nil {
		return 0, err
	}
	defer dialer.Close()

	if downloadURL == "" {
		downloadURL = DownloadLinkDefault
	}
	option := DownloadOption{
		DownloadTimeout:  timeout,
		HandshakeTimeout: handshakeTimeout,
		URL:              downloadURL,
	}
	return downloadInternal(ctx, option, resultChan, startChan, dialer.DialContext)
}

// DownloadWithURLThreads 用 threads 条并行连接下载并聚合总吞吐。每条连接在超时前不断重新
// 请求同一 URL,因此即便是很小的文件也能持续占满整个测试时长;上报给 resultChan 的每秒样本
// 是所有线程之和,返回值为峰值每秒聚合速度。threads <= 1 时退回单连接实现(保留原有快速失败行为)。
//
// ctx 为调用方上下文:客户端断开/用户点终止时取消它,本次下载(含全部 worker)会立即停止,
// 避免协程与代理连接泄漏。注意:threads 是"单个节点测速内部的并行连接数",与"并发数"(同时
// 测多少个节点)是两个不同维度。
func DownloadWithURLThreads(ctx context.Context, link string, timeout time.Duration, handshakeTimeout time.Duration, resultChan chan<- int64, startChan chan<- time.Time, downloadURL string, threads int) (int64, error) {
	dialer, err := createDialer(link)
	if err != nil {
		return 0, err
	}
	defer dialer.Close()
	if downloadURL == "" {
		downloadURL = DownloadLinkDefault
	}

	// 下载上下文派生自调用方 ctx,并叠加本次测速超时;调用方取消会一并终止下载。
	dlCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if threads <= 1 {
		option := DownloadOption{
			DownloadTimeout:  timeout,
			HandshakeTimeout: handshakeTimeout,
			URL:              downloadURL,
		}
		return downloadInternal(dlCtx, option, resultChan, startChan, dialer.DialContext)
	}

	var counter atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			transport := &http.Transport{DialContext: dialer.DialContext}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport}
			buf := pool.Get(20 * 1024)
			defer pool.Put(buf)
			for dlCtx.Err() == nil {
				req, err := http.NewRequestWithContext(dlCtx, "GET", downloadURL, nil)
				if err != nil {
					return
				}
				resp, err := client.Do(req)
				if err != nil {
					// 连接/请求失败:短暂退避后重试,直到超时或被取消
					select {
					case <-dlCtx.Done():
						return
					case <-time.After(200 * time.Millisecond):
						continue
					}
				}
				for dlCtx.Err() == nil {
					nr, er := resp.Body.Read(buf)
					if nr > 0 {
						counter.Add(int64(nr))
					}
					if er != nil {
						break
					}
				}
				resp.Body.Close()
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	start := time.Now()
	if startChan != nil {
		startChan <- start
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var max int64
	var total int64
	flush := func() {
		v := counter.Swap(0)
		if v <= 0 {
			return
		}
		total += v
		if v > max {
			max = v
		}
		if resultChan != nil {
			// 发送也受 dlCtx 约束:调用方取消/读取方退出后不会永久阻塞
			select {
			case resultChan <- v:
			case <-dlCtx.Done():
			}
		}
	}
	for {
		select {
		case <-ticker.C:
			flush()
			// 所有线程持续失败(始终 0 字节)时,最多等 ~5s 即判失败,避免对死节点空转满时长
			if total == 0 && time.Since(start) >= 5*time.Second {
				return 0, nil
			}
		case <-done:
			flush()
			return max, nil
		case <-dlCtx.Done():
			flush()
			return max, nil
		}
	}
}

func downloadInternal(ctx context.Context, option DownloadOption, resultChan chan<- int64, startOuterChan chan<- time.Time, dialContext func(ctx context.Context, network, addr string) (net.Conn, error)) (int64, error) {
	var max int64 = 0
	httpTransport := &http.Transport{}
	httpClient := &http.Client{Transport: httpTransport, Timeout: option.HandshakeTimeout}
	if dialContext != nil {
		httpTransport.DialContext = dialContext
	}
	req, err := http.NewRequestWithContext(ctx, "GET", option.URL, nil)
	if err != nil {
		return max, err
	}
	response, err := httpClient.Do(req)
	if err != nil {
		return max, err
	}
	defer response.Body.Close()
	prev := time.Now()
	if startOuterChan != nil {
		startOuterChan <- prev
	}
	var total int64
	buf := pool.Get(20 * 1024)
	defer pool.Put(buf)
	for {
		nr, er := response.Body.Read(buf)
		total += int64(nr)
		now := time.Now()
		if now.Sub(prev) >= time.Second || er != nil {
			prev = now
			if max < total {
				max = total
			}
			if resultChan != nil {
				// 发送受 ctx 约束:调用方取消/读取方退出后不会永久阻塞
				select {
				case resultChan <- total:
				case <-ctx.Done():
					return max, nil
				}
			}
			total = 0
		}
		if er != nil {
			break
		}
	}
	return max, nil
}

func DownloadComplete(link string, timeout time.Duration, handshakeTimeout time.Duration) (int64, error) {
	ctx := context.Background()
	dialer, err := createDialer(link)
	if err != nil {
		return 0, err
	}
	defer dialer.Close()
	return downloadCompleteInternal(ctx, CloudflareLink, timeout, handshakeTimeout, dialer.DialContext)
}

func downloadCompleteInternal(ctx context.Context, url string, timeout time.Duration, handshakeTimeout time.Duration, dialContext func(ctx context.Context, network, addr string) (net.Conn, error)) (int64, error) {
	var max int64 = 0
	httpTransport := &http.Transport{}
	httpClient := &http.Client{Transport: httpTransport, Timeout: handshakeTimeout}
	if dialContext != nil {
		httpTransport.DialContext = dialContext
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return max, err
	}
	response, err := httpClient.Do(req)
	if err != nil {
		return max, err
	}
	defer response.Body.Close()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	var total int64
	buf := pool.Get(20 * 1024)
	defer pool.Put(buf)
	for ctx.Err() == nil {
		nr, er := response.Body.Read(buf)
		total += int64(nr)
		if er != nil {
			break
		}
	}

	elapsedMs := time.Since(start).Milliseconds()
	if elapsedMs < 1 {
		elapsedMs = 1
	}
	max = total * 1000 / elapsedMs
	return max, nil
}

func WSDownload(link string, timeout time.Duration, handshakeTimeout time.Duration, resultChan chan<- int64) (int64, error) {
	return 0, nil
}
