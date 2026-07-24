package xray

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/xtls/xray-core/core"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/infra/conf"
)

// Dialer uses xray-core outbound to establish connections
type Dialer struct {
	config   *ProxyConfig
	instance *core.Instance
	mu       sync.Mutex
}

// NewDialer creates a new Dialer from ProxyConfig
func NewDialer(config *ProxyConfig) (*Dialer, error) {
	if config == nil {
		return nil, fmt.Errorf("proxy config is nil")
	}

	// Build JSON config for xray-core
	jsonConfig, err := buildJSONConfig(config)
	if err != nil {
		return nil, fmt.Errorf("build json config: %w", err)
	}

	// Parse JSON config (LoadConfig expects io.Reader for JSON format)
	xrayConfig, err := core.LoadConfig("json", bytes.NewReader(jsonConfig))
	if err != nil {
		return nil, fmt.Errorf("load xray config: %w", err)
	}

	// Create xray instance
	instance, err := core.New(xrayConfig)
	if err != nil {
		return nil, fmt.Errorf("create xray instance: %w", err)
	}

	// Start instance
	if err := instance.Start(); err != nil {
		instance.Close()
		return nil, fmt.Errorf("start xray instance: %w", err)
	}

	return &Dialer{
		config:   config,
		instance: instance,
	}, nil
}

// DialContext establishes a connection through the proxy
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.instance == nil {
		return nil, fmt.Errorf("dialer is closed")
	}

	// Parse destination address
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %s: %w", address, err)
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid port %s: %w", portStr, err)
	}

	// Create destination
	dest := xnet.Destination{
		Network: xnet.Network_TCP,
		Address: xnet.ParseAddress(host),
		Port:    xnet.Port(port),
	}

	if network == "udp" {
		dest.Network = xnet.Network_UDP
	}

	// Dial through xray
	return core.Dial(ctx, d.instance, dest)
}

// Close closes the dialer and releases resources
func (d *Dialer) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.instance != nil {
		if err := d.instance.Close(); err != nil {
			return err
		}
		d.instance = nil
	}
	return nil
}

// Config returns the proxy config
func (d *Dialer) Config() *ProxyConfig {
	return d.config
}

// buildJSONConfig creates a JSON config reader for xray-core
func buildJSONConfig(config *ProxyConfig) ([]byte, error) {
	// Build outbound config based on protocol
	outbound, err := buildOutboundJSON(config)
	if err != nil {
		return nil, err
	}

	// Build complete config. loglevel=none 关闭 xray-core 自身日志(否则会往 stdout 打
	// "Xray started"/deprecated 等 Warning,污染 CLI 的管道输出)。测速只把 xray 当拨号器用,
	// 其日志无价值。
	xrayConf := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "none",
		},
		"outbounds": []interface{}{outbound},
	}

	return json.Marshal(xrayConf)
}

// buildOutboundJSON builds outbound config as JSON map
func buildOutboundJSON(config *ProxyConfig) (map[string]interface{}, error) {
	outbound := map[string]interface{}{
		"tag": "proxy",
	}

	// Build settings based on protocol
	switch config.Protocol {
	case ProtocolVMess:
		outbound["protocol"] = "vmess"
		outbound["settings"] = buildVMessJSON(config)
	case ProtocolVLESS:
		outbound["protocol"] = "vless"
		outbound["settings"] = buildVLESSJSON(config)
	case ProtocolTrojan:
		outbound["protocol"] = "trojan"
		outbound["settings"] = buildTrojanJSON(config)
	case ProtocolShadowsocks:
		outbound["protocol"] = "shadowsocks"
		outbound["settings"] = buildShadowsocksJSON(config)
	case ProtocolSS2022:
		outbound["protocol"] = "shadowsocks"
		outbound["settings"] = buildSS2022JSON(config)
	case ProtocolWireGuard:
		outbound["protocol"] = "wireguard"
		outbound["settings"] = buildWireGuardJSON(config)
	case ProtocolHTTP:
		outbound["protocol"] = "http"
		outbound["settings"] = buildHTTPJSON(config)
	case ProtocolSOCKS:
		outbound["protocol"] = "socks"
		outbound["settings"] = buildSOCKSJSON(config)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", config.Protocol)
	}

	// Build stream settings
	if streamSettings := buildStreamJSON(config); streamSettings != nil {
		outbound["streamSettings"] = streamSettings
	}

	return outbound, nil
}

func buildVMessJSON(config *ProxyConfig) map[string]interface{} {
	security := config.Security
	if security == "" {
		security = "auto"
	}
	return map[string]interface{}{
		"vnext": []interface{}{
			map[string]interface{}{
				"address": config.Address,
				"port":    config.Port,
				"users": []interface{}{
					map[string]interface{}{
						"id":       config.UUID,
						"security": security,
					},
				},
			},
		},
	}
}

func buildVLESSJSON(config *ProxyConfig) map[string]interface{} {
	user := map[string]interface{}{
		"id":         config.UUID,
		"encryption": "none", // Required for VLESS
	}
	if config.Flow != "" {
		user["flow"] = config.Flow
	}
	return map[string]interface{}{
		"vnext": []interface{}{
			map[string]interface{}{
				"address": config.Address,
				"port":    config.Port,
				"users":   []interface{}{user},
			},
		},
	}
}

func buildTrojanJSON(config *ProxyConfig) map[string]interface{} {
	return map[string]interface{}{
		"servers": []interface{}{
			map[string]interface{}{
				"address":  config.Address,
				"port":     config.Port,
				"password": config.Password,
			},
		},
	}
}

func buildShadowsocksJSON(config *ProxyConfig) map[string]interface{} {
	method := config.Method
	if method == "" {
		method = "aes-256-gcm"
	}
	return map[string]interface{}{
		"servers": []interface{}{
			map[string]interface{}{
				"address":  config.Address,
				"port":     config.Port,
				"method":   method,
				"password": config.Password,
			},
		},
	}
}

func buildSS2022JSON(config *ProxyConfig) map[string]interface{} {
	return map[string]interface{}{
		"servers": []interface{}{
			map[string]interface{}{
				"address":  config.Address,
				"port":     config.Port,
				"method":   config.Method,
				"password": config.Password,
			},
		},
	}
}

func buildWireGuardJSON(config *ProxyConfig) map[string]interface{} {
	localAddr := config.LocalAddress
	if localAddr == "" {
		localAddr = "10.0.0.2/32"
	}
	mtu := config.MTU
	if mtu == 0 {
		mtu = 1420
	}

	settings := map[string]interface{}{
		"secretKey": config.PrivateKey,
		"address":   []string{localAddr},
		"mtu":       mtu,
		"peers": []interface{}{
			map[string]interface{}{
				"publicKey":  config.PeerPublicKey,
				"endpoint":   fmt.Sprintf("%s:%d", config.Address, config.Port),
				"allowedIPs": []string{"0.0.0.0/0", "::/0"},
				"keepAlive":  25,
			},
		},
	}

	if config.PreSharedKey != "" {
		settings["peers"].([]interface{})[0].(map[string]interface{})["preSharedKey"] = config.PreSharedKey
	}

	if len(config.Reserved) > 0 {
		settings["reserved"] = config.Reserved
	}

	return settings
}

func buildHTTPJSON(config *ProxyConfig) map[string]interface{} {
	server := map[string]interface{}{
		"address": config.Address,
		"port":    config.Port,
	}
	if config.Username != "" {
		server["users"] = []interface{}{
			map[string]interface{}{
				"user": config.Username,
				"pass": config.Password,
			},
		}
	}
	return map[string]interface{}{
		"servers": []interface{}{server},
	}
}

func buildSOCKSJSON(config *ProxyConfig) map[string]interface{} {
	server := map[string]interface{}{
		"address": config.Address,
		"port":    config.Port,
	}
	if config.Username != "" {
		server["users"] = []interface{}{
			map[string]interface{}{
				"user": config.Username,
				"pass": config.Password,
			},
		}
	}
	return map[string]interface{}{
		"servers": []interface{}{server},
	}
}

func buildStreamJSON(config *ProxyConfig) map[string]interface{} {
	if config.Stream == nil {
		return nil
	}

	stream := config.Stream
	settings := map[string]interface{}{}

	// Network
	network := stream.Network
	if network == "" {
		network = "tcp"
	}
	settings["network"] = network

	// Security
	security := stream.Security
	if security == "" {
		security = "none"
	}
	settings["security"] = security

	// TLS settings
	if security == "tls" && stream.TLS != nil {
		tlsSettings := map[string]interface{}{}
		if stream.TLS.ServerName != "" {
			tlsSettings["serverName"] = stream.TLS.ServerName
		}
		if len(stream.TLS.ALPN) > 0 {
			tlsSettings["alpn"] = stream.TLS.ALPN
		}
		// xray-core v26 起移除了 "allowInsecure":2026-06-01 之后 core.LoadConfig 会直接拒绝
		// 含该字段的配置(见 infra/conf/transport_internet.go 的 TLSConfig.Build)。因此不再
		// 输出 allowInsecure,改为依赖 serverName 走标准 TLS 校验:证书有效的节点(即便订阅
		// 标注了 skip-cert-verify)仍可测试,真正自签名的节点则无法再跳过校验。订阅的原始意图
		// 仍保留在 stream.TLS.AllowInsecure 字段中,供未来扩展(如证书 pin)使用。
		_ = stream.TLS.AllowInsecure
		if stream.TLS.Fingerprint != "" {
			tlsSettings["fingerprint"] = stream.TLS.Fingerprint
		}
		settings["tlsSettings"] = tlsSettings
	}

	// Reality settings
	if security == "reality" && stream.Reality != nil {
		realitySettings := map[string]interface{}{}
		if stream.Reality.ServerName != "" {
			realitySettings["serverName"] = stream.Reality.ServerName
		}
		if stream.Reality.PublicKey != "" {
			realitySettings["publicKey"] = stream.Reality.PublicKey
		}
		if stream.Reality.ShortID != "" {
			realitySettings["shortId"] = stream.Reality.ShortID
		}
		if stream.Reality.SpiderX != "" {
			realitySettings["spiderX"] = stream.Reality.SpiderX
		}
		if stream.Reality.Fingerprint != "" {
			realitySettings["fingerprint"] = stream.Reality.Fingerprint
		}
		settings["realitySettings"] = realitySettings
	}

	// Transport settings
	switch network {
	case "ws", "websocket":
		settings["network"] = "ws"
		if stream.WS != nil {
			wsSettings := map[string]interface{}{}
			if stream.WS.Path != "" {
				wsSettings["path"] = stream.WS.Path
			}
			if stream.WS.Host != "" {
				wsSettings["host"] = stream.WS.Host
			}
			if len(stream.WS.Headers) > 0 {
				wsSettings["headers"] = stream.WS.Headers
			}
			settings["wsSettings"] = wsSettings
		}
	case "grpc", "gun":
		settings["network"] = "grpc"
		if stream.GRPC != nil {
			grpcSettings := map[string]interface{}{}
			if stream.GRPC.ServiceName != "" {
				grpcSettings["serviceName"] = stream.GRPC.ServiceName
			}
			if stream.GRPC.MultiMode {
				grpcSettings["multiMode"] = true
			}
			settings["grpcSettings"] = grpcSettings
		}
	case "h2", "http":
		settings["network"] = "h2"
		if stream.H2 != nil {
			h2Settings := map[string]interface{}{}
			if stream.H2.Path != "" {
				h2Settings["path"] = stream.H2.Path
			}
			if len(stream.H2.Host) > 0 {
				h2Settings["host"] = stream.H2.Host
			}
			settings["httpSettings"] = h2Settings
		}
	case "kcp", "mkcp":
		settings["network"] = "kcp"
		if stream.KCP != nil {
			kcpSettings := map[string]interface{}{}
			if stream.KCP.HeaderType != "" {
				kcpSettings["header"] = map[string]string{"type": stream.KCP.HeaderType}
			}
			if stream.KCP.Seed != "" {
				kcpSettings["seed"] = stream.KCP.Seed
			}
			settings["kcpSettings"] = kcpSettings
		}
	case "httpupgrade":
		settings["network"] = "httpupgrade"
		if stream.HTTPUpgrade != nil {
			huSettings := map[string]interface{}{}
			if stream.HTTPUpgrade.Path != "" {
				huSettings["path"] = stream.HTTPUpgrade.Path
			}
			if stream.HTTPUpgrade.Host != "" {
				huSettings["host"] = stream.HTTPUpgrade.Host
			}
			settings["httpupgradeSettings"] = huSettings
		}
	case "splithttp", "xhttp":
		settings["network"] = "splithttp"
		if stream.SplitHTTP != nil {
			shSettings := map[string]interface{}{}
			if stream.SplitHTTP.Path != "" {
				shSettings["path"] = stream.SplitHTTP.Path
			}
			if stream.SplitHTTP.Host != "" {
				shSettings["host"] = stream.SplitHTTP.Host
			}
			if stream.SplitHTTP.Mode != "" {
				shSettings["mode"] = stream.SplitHTTP.Mode
			}
			settings["splithttpSettings"] = shSettings
		}
	case "tcp":
		if stream.TCP != nil && stream.TCP.HeaderType == "http" {
			tcpSettings := map[string]interface{}{
				"header": map[string]interface{}{
					"type": "http",
					"request": map[string]interface{}{
						"path": []string{stream.TCP.Path},
						"headers": map[string]interface{}{
							"Host": []string{stream.TCP.Host},
						},
					},
				},
			}
			settings["tcpSettings"] = tcpSettings
		}
	}

	return settings
}

// Unused import placeholder
var _ = conf.Config{}
