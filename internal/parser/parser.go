// Package parser provides unified proxy link parsing
package parser

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

// 支持的协议正则
var linkRegex = regexp.MustCompile(`(?i)^(vmess|vless|trojan|ss|shadowsocks|wireguard|wg|http|https|socks5?)://`)

// Parser interface for parsing proxy links
type Parser interface {
	// Parse parses a single link and returns ProxyConfig
	Parse(link string) (*xray.ProxyConfig, error)
	// CanParse returns true if this parser can handle the link
	CanParse(link string) bool
}

// 注册所有解析器
var parsers = []Parser{
	&VMess{},
	&VLESS{},
	&Trojan{},
	&Shadowsocks{},
	&WireGuard{},
	&HTTP{},
	&SOCKS{},
}

// ParseLink parses a single proxy link
func ParseLink(link string) (*xray.ProxyConfig, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return nil, fmt.Errorf("empty link")
	}

	for _, p := range parsers {
		if p.CanParse(link) {
			return p.Parse(link)
		}
	}

	return nil, fmt.Errorf("unsupported link format: %s", truncateLink(link))
}

// ParseLinks parses multiple links (newline separated)
func ParseLinks(input string) ([]*xray.ProxyConfig, error) {
	lines := strings.Split(input, "\n")
	var configs []*xray.ProxyConfig

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		config, err := ParseLink(line)
		if err != nil {
			continue // Skip invalid links
		}
		configs = append(configs, config)
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no valid proxy links found")
	}

	return configs, nil
}

// ParseSubscription parses subscription from URL, file path, or base64 encoded string
func ParseSubscription(input string) ([]*xray.ProxyConfig, error) {
	input = strings.TrimSpace(input)

	// Check if it's a URL
	if isURL(input) {
		return parseSubscriptionURL(input)
	}

	// Check if it's a file path
	if isFilePath(input) {
		return parseSubscriptionFile(input)
	}

	// Try to decode as base64
	if decoded, err := decodeBase64(input); err == nil {
		return ParseLinks(decoded)
	}

	// Try to parse directly as links
	return ParseLinks(input)
}

// parseSubscriptionURL fetches and parses subscription from URL
func parseSubscriptionURL(url string) ([]*xray.ProxyConfig, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch subscription: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read subscription: %w", err)
	}

	content := string(data)

	// Check if it's Clash YAML format
	if strings.Contains(content, "proxies:") {
		return parseClashYAML(content)
	}

	// Try to decode as base64
	if decoded, err := decodeBase64(content); err == nil {
		content = decoded
	}

	return ParseLinks(content)
}

// parseSubscriptionFile reads and parses subscription from file
func parseSubscriptionFile(path string) ([]*xray.ProxyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	content := string(data)

	// Check if it's Clash YAML format
	if strings.HasSuffix(strings.ToLower(path), ".yaml") ||
		strings.HasSuffix(strings.ToLower(path), ".yml") ||
		strings.Contains(content, "proxies:") {
		return parseClashYAML(content)
	}

	// Try to decode as base64
	if decoded, err := decodeBase64(content); err == nil {
		content = decoded
	}

	return ParseLinks(content)
}

// isURL checks if input is a URL
func isURL(input string) bool {
	return strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://")
}

// isFilePath checks if input is a file path
func isFilePath(input string) bool {
	if len(input) > 1024 {
		return false
	}
	if linkRegex.MatchString(input) {
		return false
	}
	_, err := os.Stat(input)
	return err == nil
}

// decodeBase64 decodes base64 string (tries multiple encodings)
func decodeBase64(s string) (string, error) {
	s = strings.TrimSpace(s)

	// Try standard base64
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}

	// Try URL-safe base64
	if decoded, err := base64.URLEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}

	// Try raw standard base64
	if decoded, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}

	// Try raw URL-safe base64
	if decoded, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}

	return "", fmt.Errorf("not a valid base64 string")
}

// truncateLink truncates link for error messages
func truncateLink(link string) string {
	if len(link) > 50 {
		return link[:50] + "..."
	}
	return link
}

