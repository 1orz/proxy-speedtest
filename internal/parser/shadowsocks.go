package parser

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

// Shadowsocks parser for ss:// links
type Shadowsocks struct{}

// CanParse checks if link is a shadowsocks link
func (s *Shadowsocks) CanParse(link string) bool {
	lower := strings.ToLower(link)
	return strings.HasPrefix(lower, "ss://") || strings.HasPrefix(lower, "shadowsocks://")
}

// Parse parses shadowsocks link to ProxyConfig
// Supports multiple formats:
// - SIP002: ss://base64(method:password)@host:port#remarks
// - Legacy: ss://base64(method:password@host:port)#remarks
// - Plain: ss://method:password@host:port#remarks
func (s *Shadowsocks) Parse(link string) (*xray.ProxyConfig, error) {
	// Normalize scheme
	link = strings.Replace(link, "shadowsocks://", "ss://", 1)

	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("parse ss url: %w", err)
	}

	if u.Scheme != "ss" {
		return nil, fmt.Errorf("not a shadowsocks link")
	}

	var method, password, host string
	var port uint16

	userInfo := u.User.String()

	if userInfo != "" && u.Host != "" {
		// SIP002 format: ss://base64(method:password)@host:port
		decoded, err := decodeBase64Flexible(userInfo)
		if err == nil {
			userInfo = decoded
		}

		parts := strings.SplitN(userInfo, ":", 2)
		if len(parts) == 2 {
			method = parts[0]
			password = parts[1]
		} else {
			// Maybe it's just base64 encoded method:password without the colon
			method = "aes-256-gcm"
			password = userInfo
		}

		host = u.Hostname()
		p, _ := strconv.ParseUint(u.Port(), 10, 16)
		port = uint16(p)
	} else {
		// Legacy format: ss://base64(method:password@host:port)
		encoded := strings.TrimPrefix(link, "ss://")
		// Remove fragment
		if idx := strings.Index(encoded, "#"); idx != -1 {
			encoded = encoded[:idx]
		}

		decoded, err := decodeBase64Flexible(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode ss link: %w", err)
		}

		// Parse method:password@host:port
		atIdx := strings.LastIndex(decoded, "@")
		if atIdx == -1 {
			return nil, fmt.Errorf("invalid ss format: missing @")
		}

		userPart := decoded[:atIdx]
		hostPart := decoded[atIdx+1:]

		parts := strings.SplitN(userPart, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid ss format: missing method:password")
		}
		method = parts[0]
		password = parts[1]

		h, p, err := net.SplitHostPort(hostPart)
		if err != nil {
			return nil, fmt.Errorf("invalid ss format: %w", err)
		}
		host = h
		portVal, _ := strconv.ParseUint(p, 10, 16)
		port = uint16(portVal)
	}

	if port == 0 {
		return nil, fmt.Errorf("invalid port")
	}

	// Determine if it's SS2022
	protocol := xray.ProtocolShadowsocks
	if strings.HasPrefix(method, "2022-") {
		protocol = xray.ProtocolSS2022
	}

	config := &xray.ProxyConfig{
		Protocol: protocol,
		Tag:      u.Fragment,
		Address:  host,
		Port:     port,
		Method:   method,
		Password: password,
	}

	if config.Tag == "" {
		config.Tag = host
	}

	// Parse additional parameters
	query := u.Query()

	// Plugin support (not fully implemented)
	if plugin := query.Get("plugin"); plugin != "" {
		// Plugins like obfs-local, v2ray-plugin etc.
		// For now, just note it but don't configure transport
	}

	return config, nil
}

// decodeBase64Flexible tries multiple base64 encodings
func decodeBase64Flexible(s string) (string, error) {
	s = strings.TrimSpace(s)

	// Pad if necessary
	if padding := len(s) % 4; padding > 0 {
		s += strings.Repeat("=", 4-padding)
	}

	// Try standard base64
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}

	// Try URL-safe base64
	if decoded, err := base64.URLEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}

	// Try raw standard base64
	s = strings.TrimRight(s, "=")
	if decoded, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}

	// Try raw URL-safe base64
	if decoded, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}

	return "", fmt.Errorf("not a valid base64 string")
}

