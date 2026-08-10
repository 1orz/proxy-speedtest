package request

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"time"

	"github.com/1orz/proxy-speedtest/internal/parser"
	"github.com/1orz/proxy-speedtest/internal/xray"
	"github.com/1orz/proxy-speedtest/utils"
)

const (
	remoteHost  = "clients3.google.com"
	pingSamples = 5 // 每次 ping 的采样次数(预热之外),取中位数以忽略异常样本

	// 测速端点延迟探测(PingLinkEndpoint)的预算:预热要承担建连 + TLS 握手(还可能
	// 换一种方法重试一次),给得宽些;之后的样本复用 keep-alive,按 tcpTimeout 就够。
	// endpointPingBudget 是整轮探测的硬上限,端点异常时不至于长时间占住这一步。
	endpointPingSamples = 3
	endpointPingWarmup  = 3 * time.Second
	endpointPingBudget  = 8 * time.Second

	pingUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/85.0.4183.121 Safari/537.36"
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

// PingLinkEndpoint 测「经 link 出站、到 endpointURL」的 HTTP 往返延迟,取中位数。
//
// 与 PingLink 的区别:PingLink 打的是固定探测站 clients3.google.com/generate_204,
// 测的是「这条代理通不通」;而直连测速没有代理可言,该探测站在不少网络下直连本就
// 不可达,失败会连带跳过整个测速。此时有意义的延迟是到测速端点自身的 RTT ——
// 这也正是普通带宽测速工具展示的 latency。
//
// headers 与真实下载请求一致(如自建端点的鉴权头),让探测走和测速同一条路径。
func PingLinkEndpoint(link string, endpointURL string, headers map[string]string) (int64, error) {
	if _, err := utils.CheckLink(link); err != nil {
		return 0, err
	}
	config, err := parser.ParseLink(link)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), endpointPingBudget)
	defer cancel()
	return HTTPPingConfig(ctx, config, endpointURL, headers, endpointPingSamples)
}

// HTTPPingConfig 通过 config 的出站对 endpointURL 连发 samples 次 HEAD,计量「发出请求
// → 收到响应头」的耗时,取中位数(毫秒,下限 1ms —— 上层用 <1 表示失败)。
//
// 必须量真实往返而不是建连耗时:xray 的 core.Dial 返回的是管道连接,握手在后台异步
// 完成,DialContext 会立刻返回,拿它计时永远接近 0。首个请求作预热(承担建连 + TLS
// 握手)后丢弃,其余样本复用 keep-alive 连接,量到的才是纯 RTT。
//
// 只要拿到 HTTP 响应就算一次有效往返(4xx/5xx 也算):端点是否鉴权通过与延迟无关。
func HTTPPingConfig(ctx context.Context, config *xray.ProxyConfig, endpointURL string, headers map[string]string, samples int) (int64, error) {
	dialer, err := xray.NewDialer(config)
	if err != nil {
		slog.Debug("endpoint ping: new dialer failed", "tag", config.Tag, "err", err)
		return 0, err
	}
	defer dialer.Close()

	transport := &http.Transport{DialContext: dialer.DialContext}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}

	probe := func(method string, byteRange bool, timeout time.Duration) (int64, error) {
		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		req, err := http.NewRequestWithContext(reqCtx, method, endpointURL, nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("User-Agent", pingUserAgent)
		if byteRange {
			req.Header.Set("Range", "bytes=0-0")
		}
		for k, v := range headers {
			if k != "" && v != "" {
				req.Header.Set(k, v)
			}
		}
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		elapse := time.Since(start).Milliseconds()
		// HEAD / 单字节 Range 的 body 本就极小;限量读干净再关,让连接能被 keep-alive
		// 复用 —— 后续样本才量得到纯 RTT 而不是又一次握手。
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if elapse < 1 {
			elapse = 1
		}
		return elapse, nil
	}

	// 预热:承担建连与 TLS 握手,同时定下这个端点吃哪种探测方法。HEAD 最省,但支持度
	// 并不普遍(部分静态文件服务器收到 HEAD 直接断开),失败就退到只取 1 字节的 Range GET。
	var chosen func(time.Duration) (int64, error)
	var lastErr error
	for _, m := range []struct {
		method    string
		byteRange bool
	}{{http.MethodHead, false}, {http.MethodGet, true}} {
		if _, err := probe(m.method, m.byteRange, endpointPingWarmup); err != nil {
			lastErr = err
			slog.Debug("endpoint ping: warmup failed", "url", endpointURL, "method", m.method, "err", err)
			continue
		}
		method, byteRange := m.method, m.byteRange
		chosen = func(d time.Duration) (int64, error) { return probe(method, byteRange, d) }
		break
	}
	if chosen == nil {
		return 0, lastErr
	}

	if samples < 1 {
		samples = 1
	}
	results := make([]int64, 0, samples)
	for i := 0; i < samples; i++ {
		if ctx.Err() != nil {
			break
		}
		elapse, err := chosen(tcpTimeout)
		if err != nil {
			lastErr = err
			continue
		}
		results = append(results, elapse)
	}
	if len(results) == 0 {
		if lastErr == nil {
			lastErr = errors.New("no successful endpoint ping sample")
		}
		return 0, lastErr
	}
	sort.Slice(results, func(i, j int) bool { return results[i] < results[j] })
	return results[len(results)/2], nil
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
