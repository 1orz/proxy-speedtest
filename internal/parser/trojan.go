package parser

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

// Trojan parser for trojan:// links
type Trojan struct{}

// CanParse checks if link is a trojan link
func (t *Trojan) CanParse(link string) bool {
	return strings.HasPrefix(strings.ToLower(link), "trojan://")
}

// Parse parses trojan link to ProxyConfig
// Format: trojan://password@host:port?sni=xxx&type=tcp#remarks
func (t *Trojan) Parse(link string) (*xray.ProxyConfig, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("parse trojan url: %w", err)
	}

	if u.Scheme != "trojan" {
		return nil, fmt.Errorf("not a trojan link")
	}

	port, _ := strconv.ParseUint(u.Port(), 10, 16)
	if port == 0 {
		port = 443
	}

	// Password is in the user part
	password := u.User.Username()

	config := &xray.ProxyConfig{
		Protocol: xray.ProtocolTrojan,
		Tag:      u.Fragment,
		Address:  u.Hostname(),
		Port:     uint16(port),
		Password: password,
	}

	if config.Tag == "" {
		config.Tag = config.Address
	}

	// Parse query parameters
	query := u.Query()

	// Build stream settings (Trojan always uses TLS by default)
	stream := &xray.StreamSettings{
		Network:  query.Get("type"),
		Security: "tls",
	}

	if stream.Network == "" {
		stream.Network = "tcp"
	}

	// TLS settings
	sni := query.Get("sni")
	if sni == "" {
		sni = query.Get("peer")
	}
	if sni == "" {
		sni = config.Address
	}

	stream.TLS = &xray.TLSSettings{
		ServerName:    sni,
		AllowInsecure: query.Get("allowInsecure") == "1" || query.Get("skip-cert-verify") == "true",
	}

	if alpn := query.Get("alpn"); alpn != "" {
		stream.TLS.ALPN = strings.Split(alpn, ",")
	}

	if fp := query.Get("fp"); fp != "" {
		stream.TLS.Fingerprint = fp
	}

	// Check for Reality
	if query.Get("security") == "reality" {
		stream.Security = "reality"
		stream.Reality = &xray.RealitySettings{
			ServerName:  sni,
			PublicKey:   query.Get("pbk"),
			ShortID:     query.Get("sid"),
			SpiderX:     query.Get("spx"),
			Fingerprint: query.Get("fp"),
		}
		stream.TLS = nil
	}

	// Transport settings
	switch stream.Network {
	case "ws", "websocket":
		stream.Network = "ws"
		stream.WS = &xray.WSSettings{
			Path: query.Get("path"),
			Host: query.Get("host"),
		}
	case "grpc", "gun":
		stream.Network = "grpc"
		serviceName := query.Get("serviceName")
		if serviceName == "" {
			serviceName = query.Get("service-name")
		}
		stream.GRPC = &xray.GRPCSettings{
			ServiceName: serviceName,
		}
	case "h2", "http":
		stream.Network = "h2"
		stream.H2 = &xray.H2Settings{
			Path: query.Get("path"),
		}
		if host := query.Get("host"); host != "" {
			stream.H2.Host = strings.Split(host, ",")
		}
	}

	config.Stream = stream
	return config, nil
}

