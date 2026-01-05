package parser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

// VMess parser for vmess:// links
type VMess struct{}

// CanParse checks if link is a vmess link
func (v *VMess) CanParse(link string) bool {
	return strings.HasPrefix(strings.ToLower(link), "vmess://")
}

// Parse parses vmess link to ProxyConfig
func (v *VMess) Parse(link string) (*xray.ProxyConfig, error) {
	// Remove prefix
	encoded := strings.TrimPrefix(link, "vmess://")
	encoded = strings.TrimPrefix(encoded, "VMESS://")

	// Check if it's Shadowrocket format (vmess://user@host:port?...)
	if strings.Contains(encoded, "@") && !strings.Contains(encoded, "{") {
		return v.parseShadowrocket(link)
	}

	// Standard V2RayN format (base64 encoded JSON)
	decoded, err := decodeBase64(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode vmess link: %w", err)
	}

	return v.parseV2RayN(decoded)
}

// vmessJSON represents V2RayN vmess link JSON format
type vmessJSON struct {
	V    interface{} `json:"v"`
	Ps   string      `json:"ps"`   // Remarks
	Add  string      `json:"add"`  // Server address
	Port interface{} `json:"port"` // Port (can be string or number)
	ID   string      `json:"id"`   // UUID
	Aid  interface{} `json:"aid"`  // AlterID (deprecated, can be string or number)
	Scy  string      `json:"scy"`  // Security (encryption)
	Net  string      `json:"net"`  // Network type
	Type string      `json:"type"` // Header type (none, http, etc)
	Host string      `json:"host"` // Host for ws/h2
	Path string      `json:"path"` // Path for ws/h2
	TLS  string      `json:"tls"`  // TLS (tls or empty)
	SNI  string      `json:"sni"`  // Server name
	ALPN string      `json:"alpn"` // ALPN
	FP   string      `json:"fp"`   // Fingerprint
}

// parseV2RayN parses V2RayN format vmess JSON
func (v *VMess) parseV2RayN(jsonStr string) (*xray.ProxyConfig, error) {
	var data vmessJSON
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("parse vmess json: %w", err)
	}

	port := parsePort(data.Port)
	if port == 0 {
		return nil, fmt.Errorf("invalid port")
	}

	config := &xray.ProxyConfig{
		Protocol: xray.ProtocolVMess,
		Tag:      data.Ps,
		Address:  data.Add,
		Port:     port,
		UUID:     data.ID,
		Security: data.Scy,
	}

	if config.Security == "" {
		config.Security = "auto"
	}

	if config.Tag == "" {
		config.Tag = data.Add
	}

	// Build stream settings
	stream := &xray.StreamSettings{
		Network:  data.Net,
		Security: "none",
	}

	if data.TLS == "tls" {
		stream.Security = "tls"
		stream.TLS = &xray.TLSSettings{
			ServerName: data.SNI,
		}
		if data.ALPN != "" {
			stream.TLS.ALPN = strings.Split(data.ALPN, ",")
		}
		if data.FP != "" {
			stream.TLS.Fingerprint = data.FP
		}
	}

	// Transport settings
	switch data.Net {
	case "ws", "websocket":
		stream.Network = "ws"
		stream.WS = &xray.WSSettings{
			Path: data.Path,
			Host: data.Host,
		}
	case "h2", "http":
		stream.Network = "h2"
		stream.H2 = &xray.H2Settings{
			Path: data.Path,
		}
		if data.Host != "" {
			stream.H2.Host = strings.Split(data.Host, ",")
		}
	case "grpc", "gun":
		stream.Network = "grpc"
		stream.GRPC = &xray.GRPCSettings{
			ServiceName: data.Path,
		}
	case "tcp":
		stream.Network = "tcp"
		if data.Type == "http" {
			stream.TCP = &xray.TCPSettings{
				HeaderType: "http",
				Host:       data.Host,
				Path:       data.Path,
			}
		}
	case "kcp", "mkcp":
		stream.Network = "kcp"
		stream.KCP = &xray.KCPSettings{
			HeaderType: data.Type,
		}
	default:
		if stream.Network == "" {
			stream.Network = "tcp"
		}
	}

	config.Stream = stream
	return config, nil
}

// parseShadowrocket parses Shadowrocket format vmess link
func (v *VMess) parseShadowrocket(link string) (*xray.ProxyConfig, error) {
	// Format: vmess://method:uuid@host:port?params#remarks
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("parse vmess url: %w", err)
	}

	// Parse user info (method:uuid or just uuid)
	userInfo := u.User.String()
	var uuid, security string

	if strings.Contains(userInfo, ":") {
		parts := strings.SplitN(userInfo, ":", 2)
		security = parts[0]
		uuid = parts[1]
	} else {
		// Try decoding as base64
		if decoded, err := base64.RawURLEncoding.DecodeString(userInfo); err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				security = parts[0]
				uuid = parts[1]
			} else {
				uuid = string(decoded)
			}
		} else {
			uuid = userInfo
		}
	}

	port, _ := strconv.ParseUint(u.Port(), 10, 16)
	if port == 0 {
		port = 443
	}

	config := &xray.ProxyConfig{
		Protocol: xray.ProtocolVMess,
		Tag:      u.Fragment,
		Address:  u.Hostname(),
		Port:     uint16(port),
		UUID:     uuid,
		Security: security,
	}

	if config.Security == "" {
		config.Security = "auto"
	}

	if config.Tag == "" {
		config.Tag = config.Address
	}

	// Parse query parameters
	query := u.Query()
	stream := &xray.StreamSettings{
		Network:  query.Get("type"),
		Security: "none",
	}

	if stream.Network == "" {
		stream.Network = query.Get("net")
	}
	if stream.Network == "" {
		stream.Network = "tcp"
	}

	// TLS settings
	if query.Get("tls") == "tls" || query.Get("security") == "tls" {
		stream.Security = "tls"
		stream.TLS = &xray.TLSSettings{
			ServerName: query.Get("sni"),
		}
		if alpn := query.Get("alpn"); alpn != "" {
			stream.TLS.ALPN = strings.Split(alpn, ",")
		}
		if fp := query.Get("fp"); fp != "" {
			stream.TLS.Fingerprint = fp
		}
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
		stream.GRPC = &xray.GRPCSettings{
			ServiceName: query.Get("serviceName"),
		}
		if sn := query.Get("service-name"); sn != "" {
			stream.GRPC.ServiceName = sn
		}
	}

	config.Stream = stream
	return config, nil
}

// parsePort parses port from interface{} (can be string or number)
func parsePort(v interface{}) uint16 {
	switch p := v.(type) {
	case string:
		port, _ := strconv.ParseUint(p, 10, 16)
		return uint16(port)
	case float64:
		return uint16(p)
	case int:
		return uint16(p)
	case int64:
		return uint16(p)
	default:
		return 0
	}
}

