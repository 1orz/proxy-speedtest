package request

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sort"
	"time"

	"github.com/1orz/proxy-speedtest/internal/parser"
	"github.com/1orz/proxy-speedtest/internal/xray"
	"github.com/1orz/proxy-speedtest/utils"
)

const (
	remoteHost  = "clients3.google.com"
	pingSamples = 5 // 每次 ping 的采样次数(预热之外),取中位数以忽略异常样本
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

// pingInternal 在同一条已建立的连接上先预热一次(丢弃,消除首次请求的额外开销),
// 再采样 samples 次,取成功样本的中位数;失败/超时的样本被忽略(异常剔除)。
// 采样期间给连接设置截止时间(取 ctx 截止与 tcpTimeout 的较早者),避免单次读写挂死。
func pingInternal(ctx context.Context, remoteConn net.Conn, samples int) (int64, error) {
	if remoteConn == nil {
		return 0, errors.New("connection is nil")
	}
	if samples < 1 {
		samples = 1
	}

	deadline := time.Now().Add(tcpTimeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	remoteConn.SetDeadline(deadline)
	defer remoteConn.SetDeadline(time.Time{})

	// 预热请求:消除首次请求(在已建连之上)残留的额外延迟。
	if _, err := pingOnce(remoteConn); err != nil {
		return 0, err
	}

	results := make([]int64, 0, samples)
	var lastErr error
	for i := 0; i < samples; i++ {
		if ctx.Err() != nil {
			break
		}
		elapse, err := pingOnce(remoteConn)
		if err != nil {
			lastErr = err
			continue
		}
		results = append(results, elapse)
	}
	if len(results) == 0 {
		if lastErr == nil {
			lastErr = errors.New("no successful ping sample")
		}
		return 0, lastErr
	}
	// 取中位数,忽略异常偏大/偶发的样本。
	sort.Slice(results, func(i, j int) bool { return results[i] < results[j] })
	return results[len(results)/2], nil
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
		slog.Debug("ping: new dialer failed", "tag", config.Tag, "err", err)
		return 0, err
	}
	defer dialer.Close()

	// Dial connection
	remoteConn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(remoteHost, "80"))
	if err != nil {
		slog.Debug("ping: dial failed", "tag", config.Tag, "err", err)
		return 0, err
	}
	defer remoteConn.Close()

	latency, err := pingInternal(ctx, remoteConn, pingSamples)
	if err != nil {
		slog.Debug("ping: sampling failed", "tag", config.Tag, "err", err)
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
