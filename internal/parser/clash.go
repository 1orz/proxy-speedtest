package parser

import (
	"fmt"
	"strings"

	"github.com/1orz/proxy-speedtest/internal/xray"
	"gopkg.in/yaml.v3"
)

// parseClashYAML parses Clash YAML configuration
func parseClashYAML(content string) ([]*xray.ProxyConfig, error) {
	var clash struct {
		Proxies []map[string]interface{} `yaml:"proxies"`
	}

	if err := yaml.Unmarshal([]byte(content), &clash); err != nil {
		return nil, fmt.Errorf("parse clash yaml: %w", err)
	}

	if len(clash.Proxies) == 0 {
		return nil, fmt.Errorf("no proxies found in clash config")
	}

	var configs []*xray.ProxyConfig
	for _, proxy := range clash.Proxies {
		config, err := parseClashProxy(proxy)
		if err != nil {
			continue // Skip invalid proxies
		}
		configs = append(configs, config)
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no valid proxies found in clash config")
	}

	return configs, nil
}

// parseClashProxy parses a single Clash proxy entry
func parseClashProxy(proxy map[string]interface{}) (*xray.ProxyConfig, error) {
	proxyType := getString(proxy, "type")

	switch proxyType {
	case "vmess":
		return parseClashVMess(proxy)
	case "vless":
		return parseClashVLESS(proxy)
	case "trojan":
		return parseClashTrojan(proxy)
	case "ss", "shadowsocks":
		return parseClashShadowsocks(proxy)
	case "socks5":
		return parseClashSOCKS(proxy)
	case "http", "https":
		return parseClashHTTP(proxy)
	case "wireguard":
		return parseClashWireGuard(proxy)
	default:
		return nil, fmt.Errorf("unsupported proxy type: %s", proxyType)
	}
}

func parseClashVMess(proxy map[string]interface{}) (*xray.ProxyConfig, error) {
	config := &xray.ProxyConfig{
		Protocol: xray.ProtocolVMess,
		Tag:      getString(proxy, "name"),
		Address:  getString(proxy, "server"),
		Port:     getUint16(proxy, "port"),
		UUID:     getString(proxy, "uuid"),
		Security: getString(proxy, "cipher"),
	}

	if config.Security == "" {
		config.Security = "auto"
	}

	stream := &xray.StreamSettings{
		Network:  getString(proxy, "network"),
		Security: "none",
	}
	if stream.Network == "" {
		stream.Network = "tcp"
	}

	// TLS
	if getBool(proxy, "tls") {
		stream.Security = "tls"
		stream.TLS = &xray.TLSSettings{
			ServerName:    getString(proxy, "servername"),
			AllowInsecure: getBool(proxy, "skip-cert-verify"),
		}
		if alpn := getStringSlice(proxy, "alpn"); len(alpn) > 0 {
			stream.TLS.ALPN = alpn
		}
	}

	// Transport
	switch stream.Network {
	case "ws":
		wsOpts := getMap(proxy, "ws-opts")
		stream.WS = &xray.WSSettings{
			Path: getString(wsOpts, "path"),
			Host: getString(getMap(wsOpts, "headers"), "Host"),
		}
	case "grpc":
		grpcOpts := getMap(proxy, "grpc-opts")
		stream.GRPC = &xray.GRPCSettings{
			ServiceName: getString(grpcOpts, "grpc-service-name"),
		}
	case "h2":
		h2Opts := getMap(proxy, "h2-opts")
		stream.H2 = &xray.H2Settings{
			Path: getString(h2Opts, "path"),
			Host: getStringSlice(h2Opts, "host"),
		}
	}

	config.Stream = stream
	return config, nil
}

func parseClashVLESS(proxy map[string]interface{}) (*xray.ProxyConfig, error) {
	config := &xray.ProxyConfig{
		Protocol: xray.ProtocolVLESS,
		Tag:      getString(proxy, "name"),
		Address:  getString(proxy, "server"),
		Port:     getUint16(proxy, "port"),
		UUID:     getString(proxy, "uuid"),
		Flow:     getString(proxy, "flow"),
	}

	stream := &xray.StreamSettings{
		Network:  getString(proxy, "network"),
		Security: "none",
	}
	if stream.Network == "" {
		stream.Network = "tcp"
	}

	// TLS
	if getBool(proxy, "tls") {
		stream.Security = "tls"
		stream.TLS = &xray.TLSSettings{
			ServerName:    getString(proxy, "servername"),
			AllowInsecure: getBool(proxy, "skip-cert-verify"),
			Fingerprint:   getString(proxy, "client-fingerprint"),
		}
		if alpn := getStringSlice(proxy, "alpn"); len(alpn) > 0 {
			stream.TLS.ALPN = alpn
		}
	}

	// Reality
	if realityOpts := getMap(proxy, "reality-opts"); len(realityOpts) > 0 {
		stream.Security = "reality"
		stream.Reality = &xray.RealitySettings{
			PublicKey:   getString(realityOpts, "public-key"),
			ShortID:     getString(realityOpts, "short-id"),
			Fingerprint: getString(proxy, "client-fingerprint"),
		}
		stream.TLS = nil
	}

	// Transport
	switch stream.Network {
	case "ws":
		wsOpts := getMap(proxy, "ws-opts")
		stream.WS = &xray.WSSettings{
			Path: getString(wsOpts, "path"),
			Host: getString(getMap(wsOpts, "headers"), "Host"),
		}
	case "grpc":
		grpcOpts := getMap(proxy, "grpc-opts")
		stream.GRPC = &xray.GRPCSettings{
			ServiceName: getString(grpcOpts, "grpc-service-name"),
		}
	}

	config.Stream = stream
	return config, nil
}

func parseClashTrojan(proxy map[string]interface{}) (*xray.ProxyConfig, error) {
	config := &xray.ProxyConfig{
		Protocol: xray.ProtocolTrojan,
		Tag:      getString(proxy, "name"),
		Address:  getString(proxy, "server"),
		Port:     getUint16(proxy, "port"),
		Password: getString(proxy, "password"),
	}

	stream := &xray.StreamSettings{
		Network:  getString(proxy, "network"),
		Security: "tls",
	}
	if stream.Network == "" {
		stream.Network = "tcp"
	}

	sni := getString(proxy, "sni")
	if sni == "" {
		sni = config.Address
	}

	stream.TLS = &xray.TLSSettings{
		ServerName:    sni,
		AllowInsecure: getBool(proxy, "skip-cert-verify"),
	}
	if alpn := getStringSlice(proxy, "alpn"); len(alpn) > 0 {
		stream.TLS.ALPN = alpn
	}

	// Transport
	switch stream.Network {
	case "ws":
		wsOpts := getMap(proxy, "ws-opts")
		stream.WS = &xray.WSSettings{
			Path: getString(wsOpts, "path"),
			Host: getString(getMap(wsOpts, "headers"), "Host"),
		}
	case "grpc":
		grpcOpts := getMap(proxy, "grpc-opts")
		stream.GRPC = &xray.GRPCSettings{
			ServiceName: getString(grpcOpts, "grpc-service-name"),
		}
	}

	config.Stream = stream
	return config, nil
}

func parseClashShadowsocks(proxy map[string]interface{}) (*xray.ProxyConfig, error) {
	method := getString(proxy, "cipher")
	protocol := xray.ProtocolShadowsocks
	if strings.HasPrefix(method, "2022-") {
		protocol = xray.ProtocolSS2022
	}

	return &xray.ProxyConfig{
		Protocol: protocol,
		Tag:      getString(proxy, "name"),
		Address:  getString(proxy, "server"),
		Port:     getUint16(proxy, "port"),
		Method:   method,
		Password: getString(proxy, "password"),
	}, nil
}

func parseClashSOCKS(proxy map[string]interface{}) (*xray.ProxyConfig, error) {
	return &xray.ProxyConfig{
		Protocol: xray.ProtocolSOCKS,
		Tag:      getString(proxy, "name"),
		Address:  getString(proxy, "server"),
		Port:     getUint16(proxy, "port"),
		Username: getString(proxy, "username"),
		Password: getString(proxy, "password"),
	}, nil
}

func parseClashHTTP(proxy map[string]interface{}) (*xray.ProxyConfig, error) {
	config := &xray.ProxyConfig{
		Protocol: xray.ProtocolHTTP,
		Tag:      getString(proxy, "name"),
		Address:  getString(proxy, "server"),
		Port:     getUint16(proxy, "port"),
		Username: getString(proxy, "username"),
		Password: getString(proxy, "password"),
	}

	if getBool(proxy, "tls") {
		config.Stream = &xray.StreamSettings{
			Network:  "tcp",
			Security: "tls",
			TLS: &xray.TLSSettings{
				ServerName:    getString(proxy, "sni"),
				AllowInsecure: getBool(proxy, "skip-cert-verify"),
			},
		}
	}

	return config, nil
}

func parseClashWireGuard(proxy map[string]interface{}) (*xray.ProxyConfig, error) {
	config := &xray.ProxyConfig{
		Protocol:      xray.ProtocolWireGuard,
		Tag:           getString(proxy, "name"),
		Address:       getString(proxy, "server"),
		Port:          getUint16(proxy, "port"),
		PrivateKey:    getString(proxy, "private-key"),
		PeerPublicKey: getString(proxy, "public-key"),
		PreSharedKey:  getString(proxy, "preshared-key"),
		LocalAddress:  getString(proxy, "ip"),
		MTU:           getInt(proxy, "mtu"),
	}

	// Parse reserved
	if reserved := proxy["reserved"]; reserved != nil {
		switch v := reserved.(type) {
		case []interface{}:
			config.Reserved = make([]byte, len(v))
			for i, val := range v {
				if num, ok := val.(int); ok {
					config.Reserved[i] = byte(num)
				} else if num, ok := val.(float64); ok {
					config.Reserved[i] = byte(num)
				}
			}
		}
	}

	return config, nil
}

// Helper functions for type conversion
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getUint16(m map[string]interface{}, key string) uint16 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return uint16(n)
		case int64:
			return uint16(n)
		case float64:
			return uint16(n)
		}
	}
	return 0
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key]; ok {
		if sub, ok := v.(map[string]interface{}); ok {
			return sub
		}
	}
	return make(map[string]interface{})
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key]; ok {
		switch slice := v.(type) {
		case []interface{}:
			result := make([]string, 0, len(slice))
			for _, item := range slice {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		case []string:
			return slice
		}
	}
	return nil
}

