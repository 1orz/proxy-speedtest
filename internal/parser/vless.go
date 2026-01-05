package parser

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

// VLESS parser for vless:// links
type VLESS struct{}

// CanParse checks if link is a vless link
func (v *VLESS) CanParse(link string) bool {
	return strings.HasPrefix(strings.ToLower(link), "vless://")
}

// Parse parses vless link to ProxyConfig
// Format: vless://uuid@host:port?type=tcp&security=tls&...#remarks
func (v *VLESS) Parse(link string) (*xray.ProxyConfig, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("parse vless url: %w", err)
	}

	if u.Scheme != "vless" {
		return nil, fmt.Errorf("not a vless link")
	}

	port, _ := strconv.ParseUint(u.Port(), 10, 16)
	if port == 0 {
		port = 443
	}

	config := &xray.ProxyConfig{
		Protocol: xray.ProtocolVLESS,
		Tag:      u.Fragment,
		Address:  u.Hostname(),
		Port:     uint16(port),
		UUID:     u.User.Username(),
	}

	if config.Tag == "" {
		config.Tag = config.Address
	}

	// Parse query parameters
	query := u.Query()

	// Flow (for XTLS)
	config.Flow = query.Get("flow")

	// Build stream settings
	stream := &xray.StreamSettings{
		Network:  query.Get("type"),
		Security: "none",
	}

	if stream.Network == "" {
		stream.Network = "tcp"
	}

	// Security settings
	security := query.Get("security")
	switch security {
	case "tls":
		stream.Security = "tls"
		stream.TLS = &xray.TLSSettings{
			ServerName:    query.Get("sni"),
			Fingerprint:   query.Get("fp"),
			AllowInsecure: query.Get("allowInsecure") == "1",
		}
		if alpn := query.Get("alpn"); alpn != "" {
			stream.TLS.ALPN = strings.Split(alpn, ",")
		}
	case "reality":
		stream.Security = "reality"
		stream.Reality = &xray.RealitySettings{
			ServerName:  query.Get("sni"),
			PublicKey:   query.Get("pbk"),
			ShortID:     query.Get("sid"),
			SpiderX:     query.Get("spx"),
			Fingerprint: query.Get("fp"),
		}
	}

	// Transport settings
	switch stream.Network {
	case "tcp":
		headerType := query.Get("headerType")
		if headerType == "http" {
			stream.TCP = &xray.TCPSettings{
				HeaderType: "http",
				Host:       query.Get("host"),
				Path:       query.Get("path"),
			}
		}
	case "ws", "websocket":
		stream.Network = "ws"
		stream.WS = &xray.WSSettings{
			Path: query.Get("path"),
			Host: query.Get("host"),
		}
		if ed := query.Get("ed"); ed != "" {
			edVal, _ := strconv.Atoi(ed)
			stream.WS.MaxEarlyData = edVal
		}
	case "grpc", "gun":
		stream.Network = "grpc"
		serviceName := query.Get("serviceName")
		if serviceName == "" {
			serviceName = query.Get("service-name")
		}
		stream.GRPC = &xray.GRPCSettings{
			ServiceName: serviceName,
			MultiMode:   query.Get("mode") == "multi",
		}
	case "h2", "http":
		stream.Network = "h2"
		stream.H2 = &xray.H2Settings{
			Path: query.Get("path"),
		}
		if host := query.Get("host"); host != "" {
			stream.H2.Host = strings.Split(host, ",")
		}
	case "kcp", "mkcp":
		stream.Network = "kcp"
		stream.KCP = &xray.KCPSettings{
			HeaderType: query.Get("headerType"),
			Seed:       query.Get("seed"),
		}
	case "quic":
		// QUIC is not supported in current xray-core transport
		stream.Network = "tcp"
	case "httpupgrade":
		stream.Network = "httpupgrade"
		stream.HTTPUpgrade = &xray.HTTPUpgradeSettings{
			Path: query.Get("path"),
			Host: query.Get("host"),
		}
	case "splithttp", "xhttp":
		stream.Network = "splithttp"
		stream.SplitHTTP = &xray.SplitHTTPSettings{
			Path: query.Get("path"),
			Host: query.Get("host"),
			Mode: query.Get("mode"),
		}
	}

	config.Stream = stream
	return config, nil
}

