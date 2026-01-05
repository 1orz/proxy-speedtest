package parser

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

// SOCKS parser for socks:// and socks5:// links
type SOCKS struct{}

// CanParse checks if link is a SOCKS proxy link
func (s *SOCKS) CanParse(link string) bool {
	lower := strings.ToLower(link)
	return strings.HasPrefix(lower, "socks://") ||
		strings.HasPrefix(lower, "socks5://") ||
		strings.HasPrefix(lower, "socks4://") ||
		strings.HasPrefix(lower, "socks4a://")
}

// Parse parses socks proxy link to ProxyConfig
// Format: socks5://user:pass@host:port#remarks
func (s *SOCKS) Parse(link string) (*xray.ProxyConfig, error) {
	// Normalize scheme to socks5
	link = strings.Replace(link, "socks://", "socks5://", 1)
	link = strings.Replace(link, "socks4://", "socks5://", 1)
	link = strings.Replace(link, "socks4a://", "socks5://", 1)

	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("parse socks url: %w", err)
	}

	if u.Scheme != "socks5" {
		return nil, fmt.Errorf("not a socks link")
	}

	port, _ := strconv.ParseUint(u.Port(), 10, 16)
	if port == 0 {
		port = 1080
	}

	config := &xray.ProxyConfig{
		Protocol: xray.ProtocolSOCKS,
		Tag:      u.Fragment,
		Address:  u.Hostname(),
		Port:     uint16(port),
	}

	// Parse credentials
	if u.User != nil {
		config.Username = u.User.Username()
		config.Password, _ = u.User.Password()
	}

	if config.Tag == "" {
		config.Tag = fmt.Sprintf("socks5-%s", config.Address)
	}

	return config, nil
}

