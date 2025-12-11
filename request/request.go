package request

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/1orz/proxy-speedtest/common"
	"github.com/1orz/proxy-speedtest/config"
	C "github.com/1orz/proxy-speedtest/constant"
	"github.com/1orz/proxy-speedtest/outbound"
	"github.com/1orz/proxy-speedtest/proxy/xray"
	"github.com/1orz/proxy-speedtest/utils"
)

const (
	remoteHost   = "clients3.google.com"
	generate_204 = "http://clients3.google.com/generate_204"
)

var (
	httpRequest = []byte("GET /generate_204 HTTP/1.1\r\nHost: clients3.google.com\r\nUser-Agent: Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/85.0.4183.121 Safari/537.36\r\n\r\n")
	tcpTimeout  = 2200 * time.Millisecond
)

func SetDNS(nameserver string) error {
	// DNS setting is now handled by xray-core
	return nil
}

func pingInternal(remoteConn net.Conn) (int64, error) {
	if remoteConn == nil {
		return 0, errors.New("connection is nil")
	}
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
	matches, err := utils.CheckLink(link)
	if err != nil {
		return 0, err
	}

	var option interface{}
	switch strings.ToLower(matches[1]) {
	case "vmess":
		option, err = config.VmessLinkToVmessOption(link)
	case "vless":
		option, err = xray.ParseVLESSLink(link)
	case "trojan":
		option, err = xray.ParseTrojanLink(link)
	case "ss":
		option, err = xray.ParseSSLink(link)
	default:
		return 0, common.NewError("Not Supported Link: " + matches[1])
	}

	if err != nil {
		return 0, err
	}

	var elapse int64
	if pingOption.TimeOut > 0 {
		tcpTimeout = pingOption.TimeOut
	}

	err = utils.ExponentialBackoff(pingOption.Attempts, 100).On(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
		defer cancel()
		pingChan := make(chan PingResult, 1)
		go func(pingChan chan<- PingResult) {
			start := time.Now()
			elp, err := Ping(option)
			elapsed := time.Since(start)
			if elapsed > 2000*time.Second {
				elp = 0
			}
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

func PingContext(ctx context.Context, option interface{}) (int64, error) {
	var d outbound.ContextDialer
	var err error
	meta := &C.Metadata{
		NetWork: 0,
		Type:    C.TEST,
		SrcPort: "",
		DstPort: "80",
		Host:    remoteHost,
		Timeout: tcpTimeout,
	}

	// Handle VMess (original implementation)
	if vmessOption, ok := option.(*outbound.VmessOption); ok {
		d, err = outbound.NewVmess(vmessOption)
		if err != nil {
			return 0, err
		}
	}

	// Handle xray-based protocols (VLESS, Trojan, SS)
	if xrayConfig, ok := option.(*xray.ProxyConfig); ok {
		d, err = outbound.NewXrayDialer(xrayConfig)
		if err != nil {
			return 0, err
		}
	}

	if d == nil {
		return 0, errors.New("not supported config type")
	}

	remoteConn, err := d.DialContext(ctx, meta)
	if err != nil {
		return 0, err
	}
	defer remoteConn.Close()

	return pingInternal(remoteConn)
}

func Ping(option interface{}) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tcpTimeout)
	defer cancel()
	return PingContext(ctx, option)
}

func pingHTTPClient(ctx context.Context, url string, timeout time.Duration, dialCtx func(ctx context.Context, network, addr string) (net.Conn, error)) (int64, error) {
	httpTransport := &http.Transport{}
	httpClient := &http.Client{Transport: httpTransport, Timeout: timeout}
	if dialCtx != nil {
		httpTransport.DialContext = dialCtx
	}
	defer httpClient.CloseIdleConnections()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	resp, err := httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return 0, fmt.Errorf("status: %d", resp.StatusCode)
	}
	elapse := time.Since(start)
	return elapse.Milliseconds(), nil
}

type PingOption struct {
	Attempts int
	TimeOut  time.Duration
}

type PingDelayOption struct {
	PingOption
	URL string
}
