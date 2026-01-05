package request

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/1orz/proxy-speedtest/internal/parser"
	"github.com/1orz/proxy-speedtest/internal/xray"
	"github.com/1orz/proxy-speedtest/utils"
)

const (
	remoteHost = "clients3.google.com"
)

var (
	httpRequest = []byte("GET /generate_204 HTTP/1.1\r\nHost: clients3.google.com\r\nUser-Agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/85.0.4183.121 Safari/537.36\r\n\r\n")
	tcpTimeout  = 2200 * time.Millisecond
)

func SetDNS(nameserver string) error {
	// DNS setting is now handled by xray-core
	return nil
}

// pingOnce performs a single HTTP request and returns the response time
func pingOnce(remoteConn net.Conn) (int64, error) {
	start := time.Now()
	if _, err := remoteConn.Write(httpRequest); err != nil {
		return 0, err
	}
	buf := make([]byte, 128)
	if n, err := remoteConn.Read(buf); err != nil && err != io.EOF {
		return 0, err
	} else if n < 10 {
		return 0, errors.New("read data not enough")
	} else {
		if !bytes.Contains(buf[:n], []byte("HTTP/1.1 204")) && !bytes.Contains(buf[:n], []byte("200")) {
			return 0, fmt.Errorf("unexpected response: %s", string(buf[:n]))
		}
	}
	elapse := time.Since(start)
	return elapse.Milliseconds(), nil
}

// pingInternal performs delay test with warm-up to eliminate first handshake latency
// Reference: https://github.com/clash-verge-rev/clash-verge-rev
// The first request includes DNS resolution, TCP handshake, and TLS handshake overhead.
// By doing a warm-up request first, the second request measures only the actual proxy latency.
func pingInternal(remoteConn net.Conn) (int64, error) {
	if remoteConn == nil {
		return 0, errors.New("connection is nil")
	}

	// Warm-up request: eliminate first handshake latency (DNS, TCP, TLS)
	_, err := pingOnce(remoteConn)
	if err != nil {
		return 0, err
	}

	// Actual delay measurement (connection already established)
	return pingOnce(remoteConn)
}

type PingResult struct {
	elapse int64
	err    error
}

func PingLink(link string, attempts int) (int64, error) {
	opt := PingOption{
		Attempts: attempts,
		TimeOut:  tcpTimeout,
	}
	return PingLinkInternal(link, opt)
}

func PingLinkInternal(link string, pingOption PingOption) (int64, error) {
	_, err := utils.CheckLink(link)
	if err != nil {
		return 0, err
	}

	// Parse link using the new unified parser
	config, err := parser.ParseLink(link)
	if err != nil {
		return 0, err
	}

	timeout := pingOption.TimeOut
	if timeout <= 0 {
		timeout = tcpTimeout
	}

	var elapse int64
	err = utils.ExponentialBackoff(pingOption.Attempts, 100).On(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		pingChan := make(chan PingResult, 1)
		go func(pingChan chan<- PingResult) {
			elp, err := PingConfig(ctx, config)
			pingResult := PingResult{elapse: elp, err: err}
			pingChan <- pingResult
		}(pingChan)
		for {
			select {
			case pingResult := <-pingChan:
				elapse = pingResult.elapse
				return pingResult.err
			case <-ctx.Done():
				return fmt.Errorf("ping time out")
			}
		}
	})
	return elapse, err
}

// PingConfig performs ping test on a ProxyConfig
func PingConfig(ctx context.Context, config *xray.ProxyConfig) (int64, error) {
	// Create dialer using xray-core
	dialer, err := xray.NewDialer(config)
	if err != nil {
		fmt.Printf("[DEBUG] NewDialer error for %s: %v\n", config.Tag, err)
		return 0, err
	}
	defer dialer.Close()

	// Dial connection
	remoteConn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(remoteHost, "80"))
	if err != nil {
		fmt.Printf("[DEBUG] DialContext error for %s: %v\n", config.Tag, err)
		return 0, err
	}
	defer remoteConn.Close()

	latency, err := pingInternal(remoteConn)
	if err != nil {
		fmt.Printf("[DEBUG] pingInternal error for %s: %v\n", config.Tag, err)
	}
	return latency, err
}

// Ping is deprecated, use PingConfig instead
func Ping(option interface{}) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
	defer cancel()

	// Try to convert to xray.ProxyConfig
	if config, ok := option.(*xray.ProxyConfig); ok {
		return PingConfig(ctx, config)
	}

	return 0, errors.New("unsupported config type, use xray.ProxyConfig")
}

type PingOption struct {
	Attempts int
	TimeOut  time.Duration
}

type PingDelayOption struct {
	PingOption
	URL string
}
