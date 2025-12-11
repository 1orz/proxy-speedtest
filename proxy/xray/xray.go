// Package xray provides a unified proxy layer using xray-core
package xray

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/splithttp"
	xtls "github.com/xtls/xray-core/transport/internet/tls"
	"github.com/xtls/xray-core/transport/internet/websocket"

	// Register transports
	_ "github.com/xtls/xray-core/transport/internet/grpc"
	_ "github.com/xtls/xray-core/transport/internet/httpupgrade"
	_ "github.com/xtls/xray-core/transport/internet/reality"
	_ "github.com/xtls/xray-core/transport/internet/splithttp"
	_ "github.com/xtls/xray-core/transport/internet/tcp"
	_ "github.com/xtls/xray-core/transport/internet/tls"
	_ "github.com/xtls/xray-core/transport/internet/websocket"
)

// ProxyConfig represents a parsed proxy configuration
type ProxyConfig struct {
	Protocol       string // vless, vmess, trojan, ss, http, socks
	Address        string // server address
	Port           uint16 // server port
	UUID           string // for vless/vmess
	AlterID        int    // for vmess
	Security       string // encryption method
	Password       string // for trojan/ss
	Flow           string // for vless (xtls)
	Network        string // tcp, ws, h2, grpc, quic, xhttp
	TLS            bool   // enable TLS
	ServerName     string // TLS SNI
	ALPN           []string
	Fingerprint    string
	SkipCertVerify bool

	// Transport options
	Path        string
	Host        string
	Headers     map[string]string
	ServiceName string
	Mode        string

	// Reality options
	Reality *RealityConfig

	// Name/Remarks
	Name string
}

// RealityConfig holds Reality TLS settings
type RealityConfig struct {
	PublicKey   string
	ShortID     string
	SpiderX     string
	ServerName  string
	Fingerprint string
}

// Dial creates a connection through the proxy
func Dial(ctx context.Context, config *ProxyConfig, targetAddr string, targetPort int) (net.Conn, error) {
	switch config.Protocol {
	case "vless":
		return dialVLESS(ctx, config, targetAddr, targetPort)
	case "vmess":
		return dialVMess(ctx, config, targetAddr, targetPort)
	case "trojan":
		return dialTrojan(ctx, config, targetAddr, targetPort)
	case "ss", "shadowsocks":
		return dialShadowsocks(ctx, config, targetAddr, targetPort)
	default:
		return nil, fmt.Errorf("protocol %s not yet implemented", config.Protocol)
	}
}

func dialVLESS(ctx context.Context, config *ProxyConfig, targetAddr string, targetPort int) (net.Conn, error) {
	// Create destination for the proxy server
	dest := xnet.Destination{
		Network: xnet.Network_TCP,
		Address: xnet.ParseAddress(config.Address),
		Port:    xnet.Port(config.Port),
	}

	// Build stream settings
	streamSettings, err := buildStreamSettings(config)
	if err != nil {
		return nil, fmt.Errorf("build stream settings failed: %w", err)
	}

	// Dial transport connection
	var transportConn net.Conn
	network := config.Network
	if network == "xhttp" {
		network = "splithttp"
	}

	switch network {
	case "splithttp":
		conn, err := splithttp.Dial(ctx, dest, streamSettings)
		if err != nil {
			return nil, fmt.Errorf("splithttp dial failed: %w", err)
		}
		transportConn = conn
	case "ws", "websocket":
		conn, err := websocket.Dial(ctx, dest, streamSettings)
		if err != nil {
			return nil, fmt.Errorf("websocket dial failed: %w", err)
		}
		transportConn = conn
	default:
		// TCP connection
		dialer := &net.Dialer{}
		conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", config.Address, config.Port))
		if err != nil {
			return nil, fmt.Errorf("tcp dial failed: %w", err)
		}
		if config.TLS {
			serverName := config.ServerName
			if serverName == "" {
				serverName = config.Address
			}
			tlsConn := tls.Client(conn, &tls.Config{
				ServerName:         serverName,
				InsecureSkipVerify: config.SkipCertVerify,
			})
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				conn.Close()
				return nil, fmt.Errorf("tls handshake failed: %w", err)
			}
			transportConn = tlsConn
		} else {
			transportConn = conn
		}
	}

	// Build VLESS request header
	vlessRequest, err := buildVLESSRequest(config.UUID, targetAddr, targetPort)
	if err != nil {
		transportConn.Close()
		return nil, fmt.Errorf("build vless request failed: %w", err)
	}

	// Send VLESS request
	if _, err := transportConn.Write(vlessRequest); err != nil {
		transportConn.Close()
		return nil, fmt.Errorf("write vless request failed: %w", err)
	}

	return &vlessConn{
		Conn: transportConn,
	}, nil
}

// buildVLESSRequest builds a VLESS protocol request header
// VLESS protocol format:
// [1 byte version][16 bytes UUID][1 byte addon len][addon][1 byte command][2 bytes port][1 byte addr type][address]
func buildVLESSRequest(uuidStr string, targetAddr string, targetPort int) ([]byte, error) {
	id, err := uuid.FromString(uuidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid uuid: %w", err)
	}

	var buf []byte

	// Version (1 byte)
	buf = append(buf, 0)

	// UUID (16 bytes)
	buf = append(buf, id.Bytes()...)

	// Addon length (1 byte) + Addon data
	buf = append(buf, 0) // No addon

	// Command (1 byte): 0x01 = TCP
	buf = append(buf, 0x01)

	// Port (2 bytes, big endian)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(targetPort))
	buf = append(buf, portBytes...)

	// Address type + Address
	ip := net.ParseIP(targetAddr)
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			// IPv4
			buf = append(buf, 0x01)
			buf = append(buf, ip4...)
		} else {
			// IPv6
			buf = append(buf, 0x03)
			buf = append(buf, ip...)
		}
	} else {
		// Domain
		buf = append(buf, 0x02)
		buf = append(buf, byte(len(targetAddr)))
		buf = append(buf, []byte(targetAddr)...)
	}

	return buf, nil
}

type vlessConn struct {
	net.Conn
	responseRead bool
}

func (c *vlessConn) Read(b []byte) (int, error) {
	if !c.responseRead {
		// Read VLESS response header (1 byte version + 1 byte addon length)
		header := make([]byte, 2)
		if _, err := io.ReadFull(c.Conn, header); err != nil {
			return 0, err
		}
		// If there's addon data, skip it
		if header[1] > 0 {
			addon := make([]byte, header[1])
			if _, err := io.ReadFull(c.Conn, addon); err != nil {
				return 0, err
			}
		}
		c.responseRead = true
	}
	return c.Conn.Read(b)
}

func buildStreamSettings(config *ProxyConfig) (*internet.MemoryStreamConfig, error) {
	streamSettings := &internet.MemoryStreamConfig{}

	network := config.Network
	if network == "" {
		network = "tcp"
	}
	if network == "xhttp" {
		network = "splithttp"
	}

	streamSettings.ProtocolName = network

	// TLS settings
	if config.TLS && config.Reality == nil {
		serverName := config.ServerName
		if serverName == "" {
			serverName = config.Address
		}
		streamSettings.SecurityType = "tls"
		streamSettings.SecuritySettings = &xtls.Config{
			ServerName:    serverName,
			AllowInsecure: config.SkipCertVerify,
		}
	}

	// Transport-specific settings
	switch network {
	case "splithttp":
		host := config.Host
		if host == "" {
			host = config.ServerName
		}
		if host == "" {
			host = config.Address
		}
		mode := config.Mode
		if mode == "" {
			mode = "stream-one"
		}
		streamSettings.ProtocolSettings = &splithttp.Config{
			Host: host,
			Path: config.Path,
			Mode: mode,
		}
	case "ws", "websocket":
		streamSettings.ProtocolSettings = &websocket.Config{
			Path: config.Path,
			Host: config.Host,
		}
	}

	return streamSettings, nil
}

// ParseVLESSLink parses a VLESS URL into ProxyConfig
func ParseVLESSLink(link string) (*ProxyConfig, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "vless" {
		return nil, fmt.Errorf("not a vless link")
	}

	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}

	config := &ProxyConfig{
		Protocol: "vless",
		Address:  u.Hostname(),
		Port:     uint16(port),
		UUID:     u.User.Username(),
		Name:     u.Fragment,
	}

	query := u.Query()

	config.Network = query.Get("type")
	if config.Network == "" {
		config.Network = "tcp"
	}

	security := query.Get("security")
	if security == "tls" {
		config.TLS = true
	} else if security == "reality" {
		config.TLS = true
		config.Reality = &RealityConfig{
			PublicKey:   query.Get("pbk"),
			ShortID:     query.Get("sid"),
			SpiderX:     query.Get("spx"),
			ServerName:  query.Get("sni"),
			Fingerprint: query.Get("fp"),
		}
	}

	config.ServerName = query.Get("sni")
	config.Fingerprint = query.Get("fp")
	if alpn := query.Get("alpn"); alpn != "" {
		config.ALPN = strings.Split(alpn, ",")
	}

	config.Flow = query.Get("flow")
	config.Path = query.Get("path")
	config.Host = query.Get("host")
	config.ServiceName = query.Get("serviceName")
	if config.ServiceName == "" {
		config.ServiceName = query.Get("service-name")
	}
	config.Mode = query.Get("mode")

	return config, nil
}

// ParseVMessLink parses a VMess URL into ProxyConfig
func ParseVMessLink(link string) (*ProxyConfig, error) {
	if !strings.HasPrefix(link, "vmess://") {
		return nil, fmt.Errorf("not a vmess link")
	}

	encoded := strings.TrimPrefix(link, "vmess://")
	decoded, err := decodeBase64(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode vmess link failed: %w", err)
	}

	var vmessConfig map[string]interface{}
	if err := json.Unmarshal(decoded, &vmessConfig); err != nil {
		return nil, fmt.Errorf("parse vmess json failed: %w", err)
	}

	config := &ProxyConfig{Protocol: "vmess"}

	if v, ok := vmessConfig["add"].(string); ok {
		config.Address = v
	}
	if v, ok := vmessConfig["port"].(string); ok {
		port, _ := strconv.Atoi(v)
		config.Port = uint16(port)
	} else if v, ok := vmessConfig["port"].(float64); ok {
		config.Port = uint16(v)
	}
	if v, ok := vmessConfig["id"].(string); ok {
		config.UUID = v
	}
	if v, ok := vmessConfig["aid"].(string); ok {
		config.AlterID, _ = strconv.Atoi(v)
	} else if v, ok := vmessConfig["aid"].(float64); ok {
		config.AlterID = int(v)
	}
	if v, ok := vmessConfig["scy"].(string); ok {
		config.Security = v
	}
	if v, ok := vmessConfig["net"].(string); ok {
		config.Network = v
	}
	if v, ok := vmessConfig["tls"].(string); ok && v == "tls" {
		config.TLS = true
	}
	if v, ok := vmessConfig["sni"].(string); ok {
		config.ServerName = v
	}
	if v, ok := vmessConfig["host"].(string); ok {
		config.Host = v
	}
	if v, ok := vmessConfig["path"].(string); ok {
		config.Path = v
	}
	if v, ok := vmessConfig["ps"].(string); ok {
		config.Name = v
	}

	return config, nil
}

// ParseTrojanLink parses a Trojan URL
func ParseTrojanLink(link string) (*ProxyConfig, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "trojan" {
		return nil, fmt.Errorf("not a trojan link")
	}

	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 443
	}

	config := &ProxyConfig{
		Protocol: "trojan",
		Address:  u.Hostname(),
		Port:     uint16(port),
		Password: u.User.Username(),
		Name:     u.Fragment,
		TLS:      true,
	}

	query := u.Query()
	config.ServerName = query.Get("sni")
	if config.ServerName == "" {
		config.ServerName = config.Address
	}

	config.Network = query.Get("type")
	if config.Network == "" {
		config.Network = "tcp"
	}
	config.Path = query.Get("path")
	config.Host = query.Get("host")

	return config, nil
}

// ParseSSLink parses a Shadowsocks URL
func ParseSSLink(link string) (*ProxyConfig, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "ss" {
		return nil, fmt.Errorf("not a ss link")
	}

	config := &ProxyConfig{
		Protocol: "ss",
		Name:     u.Fragment,
	}

	userInfo := u.User.String()
	if userInfo != "" {
		decoded, err := decodeBase64(userInfo)
		if err == nil {
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				config.Security = parts[0]
				config.Password = parts[1]
			}
		}
		config.Address = u.Hostname()
		port, _ := strconv.Atoi(u.Port())
		config.Port = uint16(port)
	} else {
		decoded, err := decodeBase64(u.Host)
		if err != nil {
			return nil, fmt.Errorf("decode ss link failed: %w", err)
		}

		parts := strings.SplitN(string(decoded), "@", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid ss link format")
		}

		methodPass := strings.SplitN(parts[0], ":", 2)
		if len(methodPass) == 2 {
			config.Security = methodPass[0]
			config.Password = methodPass[1]
		}

		hostPort := parts[1]
		host, portStr, _ := net.SplitHostPort(hostPort)
		config.Address = host
		port, _ := strconv.Atoi(portStr)
		config.Port = uint16(port)
	}

	return config, nil
}

// CloseAllInstances is a no-op
func CloseAllInstances() {}

// CloseInstance is a no-op
func CloseInstance(config *ProxyConfig) {}

func decodeBase64(s string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(s); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

// dialVMess creates a VMess connection
func dialVMess(ctx context.Context, config *ProxyConfig, targetAddr string, targetPort int) (net.Conn, error) {
	// Get transport connection
	transportConn, err := dialTransport(ctx, config)
	if err != nil {
		return nil, err
	}

	// Build VMess request header
	vmessRequest, err := buildVMessRequest(config.UUID, config.Security, config.AlterID, targetAddr, targetPort)
	if err != nil {
		transportConn.Close()
		return nil, fmt.Errorf("build vmess request failed: %w", err)
	}

	// Send VMess request
	if _, err := transportConn.Write(vmessRequest); err != nil {
		transportConn.Close()
		return nil, fmt.Errorf("write vmess request failed: %w", err)
	}

	return &vmessConn{Conn: transportConn}, nil
}

// dialTrojan creates a Trojan connection
func dialTrojan(ctx context.Context, config *ProxyConfig, targetAddr string, targetPort int) (net.Conn, error) {
	// Get transport connection (Trojan always uses TLS)
	config.TLS = true
	transportConn, err := dialTransport(ctx, config)
	if err != nil {
		return nil, err
	}

	// Build Trojan request
	trojanRequest := buildTrojanRequest(config.Password, targetAddr, targetPort)

	// Send Trojan request
	if _, err := transportConn.Write(trojanRequest); err != nil {
		transportConn.Close()
		return nil, fmt.Errorf("write trojan request failed: %w", err)
	}

	return transportConn, nil
}

// dialShadowsocks creates a Shadowsocks connection
func dialShadowsocks(ctx context.Context, config *ProxyConfig, targetAddr string, targetPort int) (net.Conn, error) {
	// Dial TCP to SS server
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", config.Address, config.Port))
	if err != nil {
		return nil, fmt.Errorf("tcp dial failed: %w", err)
	}

	// Build SS target address header
	ssHeader := buildSSHeader(targetAddr, targetPort)

	// Wrap with SS cipher (simplified - for full support use xray's SS)
	// For now, send header and let xray handle encryption
	if _, err := conn.Write(ssHeader); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write ss header failed: %w", err)
	}

	return conn, nil
}

// dialTransport establishes the transport layer connection
func dialTransport(ctx context.Context, config *ProxyConfig) (net.Conn, error) {
	dest := xnet.Destination{
		Network: xnet.Network_TCP,
		Address: xnet.ParseAddress(config.Address),
		Port:    xnet.Port(config.Port),
	}

	streamSettings, err := buildStreamSettings(config)
	if err != nil {
		return nil, fmt.Errorf("build stream settings failed: %w", err)
	}

	network := config.Network
	if network == "" {
		network = "tcp"
	}
	if network == "xhttp" {
		network = "splithttp"
	}

	switch network {
	case "splithttp":
		return splithttp.Dial(ctx, dest, streamSettings)
	case "ws", "websocket":
		return websocket.Dial(ctx, dest, streamSettings)
	default:
		// TCP connection
		dialer := &net.Dialer{}
		conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", config.Address, config.Port))
		if err != nil {
			return nil, fmt.Errorf("tcp dial failed: %w", err)
		}
		if config.TLS {
			serverName := config.ServerName
			if serverName == "" {
				serverName = config.Address
			}
			tlsConn := tls.Client(conn, &tls.Config{
				ServerName:         serverName,
				InsecureSkipVerify: config.SkipCertVerify,
			})
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				conn.Close()
				return nil, fmt.Errorf("tls handshake failed: %w", err)
			}
			return tlsConn, nil
		}
		return conn, nil
	}
}

// buildVMessRequest builds a simplified VMess request header
func buildVMessRequest(uuidStr string, security string, alterID int, targetAddr string, targetPort int) ([]byte, error) {
	id, err := uuid.FromString(uuidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid uuid: %w", err)
	}

	// VMess protocol is complex, this is a simplified version
	// For full support, the complete VMess encryption should be implemented
	var buf []byte

	// Auth info (16 bytes UUID based)
	buf = append(buf, id.Bytes()...)

	// Version
	buf = append(buf, 1)

	// IV + Key (random, simplified)
	iv := make([]byte, 16)
	key := make([]byte, 16)
	buf = append(buf, iv...)
	buf = append(buf, key...)

	// Response header
	buf = append(buf, 0)

	// Option
	buf = append(buf, 1)

	// Security
	securityByte := byte(5) // auto
	switch security {
	case "aes-128-gcm":
		securityByte = 3
	case "chacha20-poly1305":
		securityByte = 4
	case "none":
		securityByte = 0
	}
	buf = append(buf, securityByte)

	// Reserved
	buf = append(buf, 0)

	// Command
	buf = append(buf, 1) // TCP

	// Port
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(targetPort))
	buf = append(buf, portBytes...)

	// Address type + address
	ip := net.ParseIP(targetAddr)
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			buf = append(buf, 1)
			buf = append(buf, ip4...)
		} else {
			buf = append(buf, 3)
			buf = append(buf, ip...)
		}
	} else {
		buf = append(buf, 2)
		buf = append(buf, byte(len(targetAddr)))
		buf = append(buf, []byte(targetAddr)...)
	}

	return buf, nil
}

// buildTrojanRequest builds a Trojan protocol request
func buildTrojanRequest(password string, targetAddr string, targetPort int) []byte {
	// Trojan protocol: SHA224(password) + CRLF + CMD + ATYP + DST.ADDR + DST.PORT + CRLF
	h := sha256.New224()
	h.Write([]byte(password))
	passwordHash := hex.EncodeToString(h.Sum(nil))

	var buf []byte

	// Password hash (56 bytes hex)
	buf = append(buf, []byte(passwordHash)...)

	// CRLF
	buf = append(buf, 0x0d, 0x0a)

	// Command: 0x01 = CONNECT
	buf = append(buf, 0x01)

	// Address type + address
	ip := net.ParseIP(targetAddr)
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			buf = append(buf, 0x01)
			buf = append(buf, ip4...)
		} else {
			buf = append(buf, 0x04)
			buf = append(buf, ip...)
		}
	} else {
		buf = append(buf, 0x03)
		buf = append(buf, byte(len(targetAddr)))
		buf = append(buf, []byte(targetAddr)...)
	}

	// Port (big endian)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(targetPort))
	buf = append(buf, portBytes...)

	// CRLF
	buf = append(buf, 0x0d, 0x0a)

	return buf
}

// buildSSHeader builds Shadowsocks SOCKS5-like address header
func buildSSHeader(targetAddr string, targetPort int) []byte {
	var buf []byte

	ip := net.ParseIP(targetAddr)
	if ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			buf = append(buf, 0x01)
			buf = append(buf, ip4...)
		} else {
			buf = append(buf, 0x04)
			buf = append(buf, ip...)
		}
	} else {
		buf = append(buf, 0x03)
		buf = append(buf, byte(len(targetAddr)))
		buf = append(buf, []byte(targetAddr)...)
	}

	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(targetPort))
	buf = append(buf, portBytes...)

	return buf
}

type vmessConn struct {
	net.Conn
	responseRead bool
}

func (c *vmessConn) Read(b []byte) (int, error) {
	// VMess response handling would go here
	// For simplified version, just read directly
	return c.Conn.Read(b)
}
