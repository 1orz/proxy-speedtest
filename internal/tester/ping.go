package tester

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

const (
	pingHost = "clients3.google.com"
)

var httpRequest = []byte("GET /generate_204 HTTP/1.1\r\nHost: clients3.google.com\r\nUser-Agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36\r\n\r\n")

// pingTest performs a latency test using HTTP 204 request
func (t *Tester) pingTest(ctx context.Context, dialer *xray.Dialer) (int64, error) {
	timeout := t.options.Timeout
	if timeout > 5*time.Second {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Establish connection
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(pingHost, "80"))
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// Set deadline
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}

	// Warm-up request (eliminates first handshake latency)
	_, err = pingOnce(conn)
	if err != nil {
		return 0, err
	}

	// Actual latency measurement
	elapse, err := pingOnce(conn)
	if err != nil {
		return 0, err
	}

	return elapse, nil
}

// pingOnce performs a single HTTP request and measures response time
func pingOnce(conn net.Conn) (int64, error) {
	start := time.Now()

	// Send HTTP request
	if _, err := conn.Write(httpRequest); err != nil {
		return 0, err
	}

	// Read response
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		return 0, err
	}

	if n < 10 {
		return 0, errors.New("response too short")
	}

	// Check for valid HTTP response
	if !bytes.Contains(buf[:n], []byte("HTTP/1.1 204")) && !bytes.Contains(buf[:n], []byte("200")) {
		return 0, errors.New("unexpected response")
	}

	elapse := time.Since(start)
	return elapse.Milliseconds(), nil
}

// PingProxy pings a single proxy config and returns latency
func PingProxy(ctx context.Context, config *xray.ProxyConfig, timeout time.Duration) (int64, error) {
	dialer, err := xray.NewDialer(config)
	if err != nil {
		return 0, err
	}
	defer dialer.Close()

	t := &Tester{
		options: &Options{
			Timeout: timeout,
		},
	}

	return t.pingTest(ctx, dialer)
}

