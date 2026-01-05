package parser

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

// HTTP parser for http:// and https:// proxy links
type HTTP struct{}

// CanParse checks if link is an HTTP proxy link
func (h *HTTP) CanParse(link string) bool {
	lower := strings.ToLower(link)
	// Only match if it looks like a proxy URL (has user info or proxy port)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		u, err := url.Parse(link)
		if err != nil {
			return false
		}
		// Has user info (username:password@host)
		if u.User != nil && u.User.Username() != "" {
			return true
		}
		// Common proxy ports
		port := u.Port()
		if port == "8080" || port == "8888" || port == "3128" || port == "1080" {
			return true
		}
	}
	return false
}

// Parse parses http proxy link to ProxyConfig
// Format: http://user:pass@host:port or https://user:pass@host:port
func (h *HTTP) Parse(link string) (*xray.ProxyConfig, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("parse http url: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("not an http/https link")
	}

	port, _ := strconv.ParseUint(u.Port(), 10, 16)
	if port == 0 {
		if u.Scheme == "https" {
			port = 443
		} else {
			port = 8080
		}
	}

	config := &xray.ProxyConfig{
		Protocol: xray.ProtocolHTTP,
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
		config.Tag = fmt.Sprintf("http-%s", config.Address)
	}

	// If HTTPS, add TLS settings
	if u.Scheme == "https" {
		config.Stream = &xray.StreamSettings{
			Network:  "tcp",
			Security: "tls",
			TLS: &xray.TLSSettings{
				ServerName: config.Address,
			},
		}
	}

	return config, nil
}

