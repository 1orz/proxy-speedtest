package outbound

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"

	"github.com/sagernet/sing-vmess/vless"
	M "github.com/sagernet/sing/common/metadata"
	utls "github.com/sagernet/utls"

	C "github.com/xxf098/lite-proxy/constant"
	"github.com/xxf098/lite-proxy/stats"
	"github.com/xxf098/lite-proxy/transport/dialer"
	"github.com/xxf098/lite-proxy/transport/splithttp"
	"github.com/xxf098/lite-proxy/transport/vmess"
)

type Vless struct {
	*Base
	client *vless.Client
	option *VlessOption
}

type VlessOption struct {
	Name           string            `proxy:"name,omitempty"`
	Server         string            `proxy:"server"`
	Port           uint16            `proxy:"port"`
	UUID           string            `proxy:"uuid"`
	Flow           string            `proxy:"flow,omitempty"` // xtls-rprx-vision (not supported in basic mode)
	Network        string            `proxy:"network,omitempty"`
	TLS            bool              `proxy:"tls,omitempty"`
	UDP            bool              `proxy:"udp,omitempty"`
	SkipCertVerify bool              `proxy:"skip-cert-verify,omitempty"`
	ServerName     string            `proxy:"servername,omitempty"`
	Fingerprint    string            `proxy:"fingerprint,omitempty"`
	ALPN           []string          `proxy:"alpn,omitempty"`
	WSPath         string            `proxy:"ws-path,omitempty"`
	WSHeaders      map[string]string `proxy:"ws-headers,omitempty"`
	WSOpts         WSOptions         `proxy:"ws-opts,omitempty"`
	HTTP2Opts      HTTP2Options      `proxy:"h2-opts,omitempty"`
	GrpcOpts       GrpcOptions       `proxy:"grpc-opts,omitempty"`
	Reality        *RealityOptions   `proxy:"reality-opts,omitempty"`
	Remarks        string            `proxy:"remarks,omitempty"`
}

type RealityOptions struct {
	PublicKey string `proxy:"public-key"`
	ShortID   string `proxy:"short-id,omitempty"`
}

// StreamConn wraps the connection with VLESS protocol and transport
func (v *Vless) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	var err error

	// Handle transport layer first
	switch v.option.Network {
	case "ws":
		host, port, _ := net.SplitHostPort(v.addr)
		wsOpts := &vmess.WebsocketConfig{
			Host: host,
			Port: port,
			Path: v.option.WSPath,
		}

		if len(v.option.WSHeaders) != 0 {
			header := http.Header{}
			for key, value := range v.option.WSHeaders {
				header.Add(key, value)
			}
			wsOpts.Headers = header
		}

		if v.option.TLS && v.option.Reality == nil {
			wsOpts.TLS = true
			wsOpts.SessionCache = getClientSessionCache()
			wsOpts.SkipCertVerify = v.option.SkipCertVerify
			wsOpts.ServerName = v.option.ServerName
		}
		c, err = vmess.StreamWebsocketConn(c, wsOpts)
		if err != nil {
			return nil, err
		}

	case "h2":
		host, _, _ := net.SplitHostPort(v.addr)
		tlsOpts := vmess.TLSConfig{
			Host:           host,
			SkipCertVerify: v.option.SkipCertVerify,
			SessionCache:   getClientSessionCache(),
			NextProtos:     []string{"h2"},
		}

		if v.option.ServerName != "" {
			tlsOpts.Host = v.option.ServerName
		}

		c, err = vmess.StreamTLSConn(c, &tlsOpts)
		if err != nil {
			return nil, err
		}

		h2Opts := &vmess.H2Config{
			Hosts: v.option.HTTP2Opts.Host,
			Path:  v.option.HTTP2Opts.Path,
		}

		c, err = vmess.StreamH2Conn(c, h2Opts)
		if err != nil {
			return nil, err
		}

	case "xhttp", "splithttp":
		host, port, _ := net.SplitHostPort(v.addr)
		serverName := v.option.ServerName
		if serverName == "" {
			serverName = host
		}
		path := v.option.WSPath
		if path == "" {
			path = "/"
		}
		cfg := &splithttp.Config{
			Host: host, Path: path, ServerName: serverName,
			TLS: v.option.TLS, SkipCertVerify: v.option.SkipCertVerify,
			Headers: v.option.WSHeaders,
		}
		c.Close()
		c, err = splithttp.Dial(context.Background(), net.JoinHostPort(host, port), cfg)
		if err != nil {
			return nil, err
		}

	default: // tcp or grpc
		// Handle Reality TLS
		if v.option.Reality != nil {
			c, err = v.realityHandshake(c)
			if err != nil {
				return nil, err
			}
		} else if v.option.TLS {
			// Handle regular TLS with optional uTLS fingerprint
			if v.option.Fingerprint != "" {
				c, err = v.utlsHandshake(c)
			} else {
				host, _, _ := net.SplitHostPort(v.addr)
				tlsOpts := &vmess.TLSConfig{
					Host:           host,
					SkipCertVerify: v.option.SkipCertVerify,
					SessionCache:   getClientSessionCache(),
				}
				if v.option.ServerName != "" {
					tlsOpts.Host = v.option.ServerName
				}
				c, err = vmess.StreamTLSConn(c, tlsOpts)
			}
			if err != nil {
				return nil, err
			}
		}
	}

	// Build destination address using sing's Socksaddr
	port, _ := strconv.ParseUint(metadata.DstPort, 10, 16)
	var destination M.Socksaddr

	// Try to parse as IP first
	if ip := net.ParseIP(metadata.Host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			destination = M.SocksaddrFromNetIP(netip.AddrPortFrom(netip.AddrFrom4([4]byte(ip4)), uint16(port)))
		} else {
			var ip6 [16]byte
			copy(ip6[:], ip.To16())
			destination = M.SocksaddrFromNetIP(netip.AddrPortFrom(netip.AddrFrom16(ip6), uint16(port)))
		}
	} else {
		// Use FQDN
		destination = M.Socksaddr{
			Fqdn: metadata.Host,
			Port: uint16(port),
		}
	}

	// Wrap with VLESS protocol
	return v.client.DialConn(c, destination)
}

func (v *Vless) DialContext(ctx context.Context, metadata *C.Metadata) (net.Conn, error) {
	c, err := dialer.DialContext(ctx, "tcp", v.addr)
	if err != nil {
		return nil, fmt.Errorf("%s connect error: %w", v.addr, err)
	}
	tcpKeepAlive(c)

	if metadata.Type == C.TEST {
		if tcpconn, ok := c.(*net.TCPConn); ok {
			tcpconn.SetLinger(0)
		}
	}

	sc := stats.NewConn(c)
	return v.StreamConn(sc, metadata)
}

func (v *Vless) DialUDP(metadata *C.Metadata) (net.PacketConn, error) {
	// VLESS UDP is not implemented in this version
	return nil, fmt.Errorf("VLESS UDP not supported")
}

// realityHandshake performs Reality TLS handshake using uTLS
// Note: This is a simplified implementation. Full Reality support requires additional libraries.
func (v *Vless) realityHandshake(c net.Conn) (net.Conn, error) {
	if v.option.Reality == nil {
		return nil, fmt.Errorf("reality options not configured")
	}

	_, err := base64.RawURLEncoding.DecodeString(v.option.Reality.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid reality public key: %w", err)
	}

	if v.option.Reality.ShortID != "" {
		_, err := hex.DecodeString(v.option.Reality.ShortID)
		if err != nil {
			return nil, fmt.Errorf("invalid reality short id: %w", err)
		}
	}

	serverName := v.option.ServerName
	if serverName == "" {
		serverName = v.option.Server
	}

	// Use uTLS with Reality-like handshake
	// Note: Full Reality protocol requires the sagernet/reality package
	// This implementation uses standard uTLS which works for basic TLS fingerprinting
	fingerprint := v.option.Fingerprint
	if fingerprint == "" {
		fingerprint = "chrome"
	}

	clientHelloID := getUTLSClientHelloID(fingerprint)
	uConn := utls.UClient(c, &utls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: true,
		NextProtos:         v.option.ALPN,
	}, clientHelloID)

	if err := uConn.Handshake(); err != nil {
		return nil, fmt.Errorf("utls handshake failed: %w", err)
	}

	return uConn, nil
}

// utlsHandshake performs uTLS handshake with fingerprint
func (v *Vless) utlsHandshake(c net.Conn) (net.Conn, error) {
	serverName := v.option.ServerName
	if serverName == "" {
		host, _, _ := net.SplitHostPort(v.addr)
		serverName = host
	}

	clientHelloID := getUTLSClientHelloID(v.option.Fingerprint)

	uConn := utls.UClient(c, &utls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: v.option.SkipCertVerify,
		NextProtos:         v.option.ALPN,
	}, clientHelloID)

	if err := uConn.Handshake(); err != nil {
		return nil, fmt.Errorf("utls handshake failed: %w", err)
	}

	return uConn, nil
}

// getUTLSClientHelloID returns the uTLS ClientHelloID for the given fingerprint
func getUTLSClientHelloID(fingerprint string) utls.ClientHelloID {
	switch fingerprint {
	case "chrome":
		return utls.HelloChrome_Auto
	case "firefox":
		return utls.HelloFirefox_Auto
	case "safari":
		return utls.HelloSafari_Auto
	case "edge":
		return utls.HelloEdge_Auto
	case "ios":
		return utls.HelloIOS_Auto
	case "android":
		return utls.HelloAndroid_11_OkHttp
	case "random":
		return utls.HelloRandomized
	default:
		return utls.HelloChrome_Auto
	}
}

func NewVless(option *VlessOption) (*Vless, error) {
	// sing-vmess/vless.NewClient needs UUID, flow, and logger
	client, err := vless.NewClient(option.UUID, option.Flow, nil)
	if err != nil {
		return nil, fmt.Errorf("create vless client failed: %w", err)
	}

	addr := net.JoinHostPort(option.Server, strconv.Itoa(int(option.Port)))

	return &Vless{
		Base: &Base{
			name: option.Name,
			addr: addr,
			udp:  option.UDP,
		},
		client: client,
		option: option,
	}, nil
}
