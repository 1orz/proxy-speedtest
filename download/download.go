package download

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/1orz/proxy-speedtest/common/pool"
	"github.com/1orz/proxy-speedtest/internal/parser"
	"github.com/1orz/proxy-speedtest/internal/xray"
	"github.com/1orz/proxy-speedtest/stats"
)

const (
	DownloadLinkDefault = "https://download.microsoft.com/download/2/0/E/20E90413-712F-438C-988E-FDAA79A8AC3D/dotnetfx35.exe"
	CloudflareLink100   = "https://speed.cloudflare.com/__down?bytes=100000000"
	CloudflareLink200   = "https://speed.cloudflare.com/__down?bytes=200000000"
	CloudflareLink10    = "https://speed.cloudflare.com/__down?bytes=10000000"
	Cachefly10          = "http://cachefly.cachefly.net/10mb.test"
	Cachefly100         = "http://cachefly.cachefly.net/100mb.test"
	HetznerLink100      = "https://ash-speed.hetzner.com/100MB.bin"
	ThinkBroadband100   = "http://ipv4.download.thinkbroadband.com/100MB.zip"
)

func GetDownloadURL(size string, customURL string) string {
	if customURL != "" {
		return customURL
	}
	switch size {
	case "10mb":
		return Cachefly10
	case "100mb", "cachefly100":
		return Cachefly100
	case "cloudflare10":
		return CloudflareLink10
	case "cloudflare100":
		return CloudflareLink100
	case "cloudflare200":
		return CloudflareLink200
	case "hetzner100":
		return HetznerLink100
	case "thinkbroadband100":
		return ThinkBroadband100
	default:
		return DownloadLinkDefault
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

func downloadInternal(ctx context.Context, option DownloadOption, resultChan chan<- int64, startOuterChan chan<- time.Time, dialContext func(ctx context.Context, network, addr string) (net.Conn, error)) (int64, error) {
	var max int64 = 0
	httpTransport := &http.Transport{}
	httpClient := &http.Client{Transport: httpTransport, Timeout: option.HandshakeTimeout}
	if dialContext != nil {
		httpTransport.DialContext = dialContext
	}
	req, err := http.NewRequest("GET", option.URL, nil)
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
			if resultChan != nil {
				resultChan <- total
			}
			if max < total {
				max = total
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
	return downloadCompleteInternal(ctx, Cachefly100, timeout, handshakeTimeout, dialer.DialContext)
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

	now := time.Now()
	max = total * 1000 / now.Sub(start).Milliseconds()
	return max, nil
}

func WSDownload(link string, timeout time.Duration, handshakeTimeout time.Duration, resultChan chan<- int64) (int64, error) {
	return 0, nil
}
