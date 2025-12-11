package config

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ClashConfig represents a clash config file
type ClashConfig struct {
	Proxies []string
}

// ParseClash parses clash config
func ParseClash(data []byte) (*ClashConfig, error) {
	var raw struct {
		Proxies []map[string]interface{} `yaml:"proxies"`
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	cc := &ClashConfig{
		Proxies: make([]string, 0, len(raw.Proxies)),
	}

	for _, proxy := range raw.Proxies {
		link := proxyToLink(proxy)
		if link != "" {
			cc.Proxies = append(cc.Proxies, link)
		}
	}

	return cc, nil
}

// ParseBaseProxy parses a base proxy line
func ParseBaseProxy(line string) (string, error) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "-") {
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimSpace(line)
	}
	if strings.Contains(line, "type:") {
		return line, nil
	}
	return "", nil
}

func proxyToLink(proxy map[string]interface{}) string {
	proxyType, _ := proxy["type"].(string)
	switch proxyType {
	case "vmess":
		return vmessProxyToLink(proxy)
	case "vless":
		return vlessProxyToLink(proxy)
	case "trojan":
		return trojanProxyToLink(proxy)
	case "ss":
		return ssProxyToLink(proxy)
	}
	return ""
}

func vmessProxyToLink(proxy map[string]interface{}) string {
	return ""
}

func vlessProxyToLink(proxy map[string]interface{}) string {
	return ""
}

func trojanProxyToLink(proxy map[string]interface{}) string {
	return ""
}

func ssProxyToLink(proxy map[string]interface{}) string {
	return ""
}

// resolveIP is a stub function for DNS resolution
func resolveIP(host string) (string, error) {
	// In the current implementation, we don't resolve IP
	// Just return the original host
	return host, nil
}
