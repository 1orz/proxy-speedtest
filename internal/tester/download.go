package tester

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

// Download URLs
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

// GetDownloadURL returns the download URL based on size preset
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

// speedTest performs a download speed test
func (t *Tester) speedTest(ctx context.Context, dialer *xray.Dialer) (avgSpeed, maxSpeed, traffic int64, err error) {
	downloadURL := GetDownloadURL(t.options.DownloadSize, t.options.DownloadURL)

	// Create HTTP transport with custom dialer
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		},
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   t.options.Timeout,
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return 0, 0, 0, err
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()

	// Download and measure speed
	return measureDownloadSpeed(ctx, resp.Body, t.options.Timeout)
}

// measureDownloadSpeed reads from reader and calculates download speed
func measureDownloadSpeed(ctx context.Context, reader io.Reader, timeout time.Duration) (avgSpeed, maxSpeed, traffic int64, err error) {
	buf := make([]byte, 32*1024) // 32KB buffer
	start := time.Now()
	lastCheck := start
	var intervalBytes int64

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			// Calculate final average speed
			duration := time.Since(start)
			if duration > 0 {
				avgSpeed = traffic * 1000 / duration.Milliseconds()
			}
			return avgSpeed, maxSpeed, traffic, nil
		default:
		}

		n, readErr := reader.Read(buf)
		if n > 0 {
			traffic += int64(n)
			intervalBytes += int64(n)

			// Check speed every second
			now := time.Now()
			if now.Sub(lastCheck) >= time.Second {
				currentSpeed := intervalBytes * 1000 / now.Sub(lastCheck).Milliseconds()
				if currentSpeed > maxSpeed {
					maxSpeed = currentSpeed
				}
				intervalBytes = 0
				lastCheck = now
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			err = readErr
			break
		}
	}

	// Calculate average speed
	duration := time.Since(start)
	if duration > 0 {
		avgSpeed = traffic * 1000 / duration.Milliseconds()
	}

	return avgSpeed, maxSpeed, traffic, err
}

// DownloadTest performs a download test on a proxy config
func DownloadTest(ctx context.Context, config *xray.ProxyConfig, timeout time.Duration, downloadURL string) (avgSpeed, maxSpeed, traffic int64, err error) {
	dialer, err := xray.NewDialer(config)
	if err != nil {
		return 0, 0, 0, err
	}
	defer dialer.Close()

	t := &Tester{
		options: &Options{
			Timeout:     timeout,
			DownloadURL: downloadURL,
		},
	}

	return t.speedTest(ctx, dialer)
}

// FormatSpeed formats speed in bytes/s to human readable string
func FormatSpeed(bytesPerSec int64) string {
	const unit = 1024
	if bytesPerSec < unit {
		return formatInt(bytesPerSec) + " B/s"
	}
	div, exp := int64(unit), 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return formatFloat(float64(bytesPerSec)/float64(div)) + string("KMGTPE"[exp]) + "B/s"
}

// FormatBytes formats bytes to human readable string
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return formatInt(bytes) + " B"
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return formatFloat(float64(bytes)/float64(div)) + string("KMGTPE"[exp]) + "B"
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + formatInt(-n)
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func formatFloat(f float64) string {
	// Simple formatting with 1 decimal place
	whole := int64(f)
	frac := int64((f - float64(whole)) * 10)
	if frac < 0 {
		frac = -frac
	}
	return formatInt(whole) + "." + formatInt(frac)
}

