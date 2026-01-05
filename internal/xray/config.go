// Package xray provides a unified interface for xray-core outbound connections
package xray

// Protocol represents the proxy protocol type
type Protocol string

const (
	ProtocolVMess       Protocol = "vmess"
	ProtocolVLESS       Protocol = "vless"
	ProtocolTrojan      Protocol = "trojan"
	ProtocolShadowsocks Protocol = "shadowsocks"
	ProtocolSS2022      Protocol = "shadowsocks-2022"
	ProtocolWireGuard   Protocol = "wireguard"
	ProtocolHTTP        Protocol = "http"
	ProtocolSOCKS       Protocol = "socks"
)

// ProxyConfig represents unified proxy configuration
type ProxyConfig struct {
	Protocol Protocol `json:"protocol"`
	Tag      string   `json:"tag"`      // Node name/remarks
	Address  string   `json:"address"`  // Server address
	Port     uint16   `json:"port"`     // Server port
	Link     string   `json:"link"`     // Original link

	// VMess/VLESS specific
	UUID     string `json:"uuid,omitempty"`
	AlterID  int    `json:"alterId,omitempty"`  // VMess only
	Security string `json:"security,omitempty"` // Encryption method (auto, aes-128-gcm, chacha20-poly1305, none)
	Flow     string `json:"flow,omitempty"`     // VLESS flow (xtls-rprx-vision)

	// Trojan/Shadowsocks specific
	Password string `json:"password,omitempty"`
	Method   string `json:"method,omitempty"` // SS cipher method

	// WireGuard specific
	PrivateKey    string `json:"privateKey,omitempty"`
	PublicKey     string `json:"publicKey,omitempty"`     // Self public key (derived from private)
	PeerPublicKey string `json:"peerPublicKey,omitempty"` // Server's public key
	PreSharedKey  string `json:"preSharedKey,omitempty"`
	Reserved      []byte `json:"reserved,omitempty"`
	MTU           int    `json:"mtu,omitempty"`
	LocalAddress  string `json:"localAddress,omitempty"` // WireGuard interface address

	// HTTP/SOCKS specific
	Username string `json:"username,omitempty"`

	// Transport layer settings
	Stream *StreamSettings `json:"stream,omitempty"`
}

// StreamSettings represents transport layer configuration
type StreamSettings struct {
	Network  string `json:"network"`  // tcp, ws, grpc, h2, quic, httpupgrade, splithttp, kcp
	Security string `json:"security"` // none, tls, reality

	// TLS settings
	TLS *TLSSettings `json:"tls,omitempty"`

	// Reality settings
	Reality *RealitySettings `json:"reality,omitempty"`

	// Transport specific settings
	TCP         *TCPSettings         `json:"tcp,omitempty"`
	WS          *WSSettings          `json:"ws,omitempty"`
	GRPC        *GRPCSettings        `json:"grpc,omitempty"`
	H2          *H2Settings          `json:"h2,omitempty"`
	QUIC        *QUICSettings        `json:"quic,omitempty"`
	KCP         *KCPSettings         `json:"kcp,omitempty"`
	HTTPUpgrade *HTTPUpgradeSettings `json:"httpupgrade,omitempty"`
	SplitHTTP   *SplitHTTPSettings   `json:"splithttp,omitempty"`
}

// TLSSettings represents TLS configuration
type TLSSettings struct {
	ServerName     string   `json:"serverName,omitempty"`
	ALPN           []string `json:"alpn,omitempty"`
	AllowInsecure  bool     `json:"allowInsecure,omitempty"`
	Fingerprint    string   `json:"fingerprint,omitempty"` // uTLS fingerprint
	DisableSystemRoot bool  `json:"disableSystemRoot,omitempty"`
}

// RealitySettings represents Reality TLS configuration
type RealitySettings struct {
	ServerName  string `json:"serverName,omitempty"`
	PublicKey   string `json:"publicKey,omitempty"`
	ShortID     string `json:"shortId,omitempty"`
	SpiderX     string `json:"spiderX,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// TCPSettings represents TCP transport settings
type TCPSettings struct {
	HeaderType string            `json:"headerType,omitempty"` // none, http
	Host       string            `json:"host,omitempty"`
	Path       string            `json:"path,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// WSSettings represents WebSocket transport settings
type WSSettings struct {
	Path                string            `json:"path,omitempty"`
	Host                string            `json:"host,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
	MaxEarlyData        int               `json:"maxEarlyData,omitempty"`
	EarlyDataHeaderName string            `json:"earlyDataHeaderName,omitempty"`
}

// GRPCSettings represents gRPC transport settings
type GRPCSettings struct {
	ServiceName         string `json:"serviceName,omitempty"`
	MultiMode           bool   `json:"multiMode,omitempty"`
	IdleTimeout         int    `json:"idleTimeout,omitempty"`
	HealthCheckTimeout  int    `json:"healthCheckTimeout,omitempty"`
	PermitWithoutStream bool   `json:"permitWithoutStream,omitempty"`
}

// H2Settings represents HTTP/2 transport settings
type H2Settings struct {
	Host []string `json:"host,omitempty"`
	Path string   `json:"path,omitempty"`
}

// QUICSettings represents QUIC transport settings
type QUICSettings struct {
	Security string `json:"security,omitempty"` // none, aes-128-gcm, chacha20-poly1305
	Key      string `json:"key,omitempty"`
	Header   string `json:"header,omitempty"` // none, srtp, utp, wechat-video, dtls, wireguard
}

// KCPSettings represents mKCP transport settings
type KCPSettings struct {
	MTU              int    `json:"mtu,omitempty"`
	TTI              int    `json:"tti,omitempty"`
	UplinkCapacity   int    `json:"uplinkCapacity,omitempty"`
	DownlinkCapacity int    `json:"downlinkCapacity,omitempty"`
	Congestion       bool   `json:"congestion,omitempty"`
	ReadBufferSize   int    `json:"readBufferSize,omitempty"`
	WriteBufferSize  int    `json:"writeBufferSize,omitempty"`
	HeaderType       string `json:"headerType,omitempty"` // none, srtp, utp, wechat-video, dtls, wireguard
	Seed             string `json:"seed,omitempty"`
}

// HTTPUpgradeSettings represents HTTP Upgrade transport settings
type HTTPUpgradeSettings struct {
	Host    string            `json:"host,omitempty"`
	Path    string            `json:"path,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// SplitHTTPSettings represents SplitHTTP (XHTTP) transport settings
type SplitHTTPSettings struct {
	Host    string            `json:"host,omitempty"`
	Path    string            `json:"path,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Mode    string            `json:"mode,omitempty"` // auto, packet-up, stream-up, stream-one
}

// GetName returns the display name for this proxy config
func (c *ProxyConfig) GetName() string {
	if c.Tag != "" {
		return c.Tag
	}
	return string(c.Protocol)
}

// GetServerAddr returns the server address in host:port format
func (c *ProxyConfig) GetServerAddr() string {
	return c.Address + ":" + itoa(int(c.Port))
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

