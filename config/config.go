package config

import (
	"regexp"

	"github.com/1orz/proxy-speedtest/internal/parser"
	"github.com/1orz/proxy-speedtest/internal/xray"
)

// Config represents a simplified proxy configuration for display
type Config struct {
	Protocol string
	Remarks  string
	Server   string
	Net      string // vmess net type
	Port     int
	Password string
	SNI      string
}

// Link2Config parses a proxy link and returns a simplified config
func Link2Config(link string) (*Config, error) {
	proxyConfig, err := parser.ParseLink(link)
	if err != nil {
		return nil, err
	}

	return proxyConfigToConfig(proxyConfig), nil
}

// Link2ProxyConfig parses a link to full ProxyConfig
func Link2ProxyConfig(link string) (*xray.ProxyConfig, error) {
	return parser.ParseLink(link)
}

// ParseSubscription parses subscription and returns proxy configs
func ParseSubscription(input string) ([]*xray.ProxyConfig, error) {
	return parser.ParseSubscription(input)
}

// proxyConfigToConfig converts xray.ProxyConfig to simplified Config
func proxyConfigToConfig(pc *xray.ProxyConfig) *Config {
	cfg := &Config{
		Protocol: string(pc.Protocol),
		Remarks:  pc.Tag,
		Server:   pc.Address,
		Port:     int(pc.Port),
	}

	// Set password based on protocol
	switch pc.Protocol {
	case xray.ProtocolVMess, xray.ProtocolVLESS:
		cfg.Password = pc.UUID
	case xray.ProtocolTrojan, xray.ProtocolShadowsocks, xray.ProtocolSS2022:
		cfg.Password = pc.Password
	}

	// Set network type
	if pc.Stream != nil {
		cfg.Net = pc.Stream.Network
		if pc.Stream.TLS != nil {
			cfg.SNI = pc.Stream.TLS.ServerName
		}
		if pc.Stream.Reality != nil {
			cfg.SNI = pc.Stream.Reality.ServerName
		}
	}

	if cfg.Remarks == "" {
		cfg.Remarks = cfg.Server
		}

	return cfg
}

// RegShadowrocketVmess matches shadowrocket vmess format
// Format: vmess://method:uuid@host:port?...
var RegShadowrocketVmess = regexp.MustCompile(`(?i)^vmess://[a-z0-9-]+:[a-f0-9-]+@`)

// ShadowrocketLinkToVmessLink converts shadowrocket vmess link
// For backward compatibility - the new parser handles this automatically
func ShadowrocketLinkToVmessLink(link string) (string, error) {
	return link, nil
}
