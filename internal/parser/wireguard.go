package parser

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/1orz/proxy-speedtest/internal/xray"
)

// WireGuard parser for wireguard:// and wg:// links
type WireGuard struct{}

// CanParse checks if link is a wireguard link
func (w *WireGuard) CanParse(link string) bool {
	lower := strings.ToLower(link)
	return strings.HasPrefix(lower, "wireguard://") || strings.HasPrefix(lower, "wg://")
}

// Parse parses wireguard link to ProxyConfig
// Format: wireguard://privatekey@host:port?publickey=xxx&reserved=xxx&address=xxx&mtu=xxx#remarks
// Or: wg://privatekey@host:port?...
func (w *WireGuard) Parse(link string) (*xray.ProxyConfig, error) {
	// Normalize scheme
	link = strings.Replace(link, "wg://", "wireguard://", 1)

	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("parse wireguard url: %w", err)
	}

	if u.Scheme != "wireguard" {
		return nil, fmt.Errorf("not a wireguard link")
	}

	port, _ := strconv.ParseUint(u.Port(), 10, 16)
	if port == 0 {
		port = 51820
	}

	// Private key is in the user part
	privateKey := u.User.Username()

	config := &xray.ProxyConfig{
		Protocol:   xray.ProtocolWireGuard,
		Tag:        u.Fragment,
		Address:    u.Hostname(),
		Port:       uint16(port),
		PrivateKey: privateKey,
	}

	if config.Tag == "" {
		config.Tag = config.Address
	}

	// Parse query parameters
	query := u.Query()

	// Peer public key
	config.PeerPublicKey = query.Get("publickey")
	if config.PeerPublicKey == "" {
		config.PeerPublicKey = query.Get("public-key")
	}
	if config.PeerPublicKey == "" {
		config.PeerPublicKey = query.Get("peer")
	}

	// Pre-shared key (optional)
	config.PreSharedKey = query.Get("presharedkey")
	if config.PreSharedKey == "" {
		config.PreSharedKey = query.Get("psk")
	}

	// Local address (WireGuard interface address)
	config.LocalAddress = query.Get("address")
	if config.LocalAddress == "" {
		config.LocalAddress = query.Get("local-address")
	}
	if config.LocalAddress == "" {
		config.LocalAddress = "10.0.0.2/32"
	}

	// MTU
	if mtu := query.Get("mtu"); mtu != "" {
		mtuVal, _ := strconv.Atoi(mtu)
		config.MTU = mtuVal
	}
	if config.MTU == 0 {
		config.MTU = 1420
	}

	// Reserved bytes (for Cloudflare WARP, etc.)
	if reserved := query.Get("reserved"); reserved != "" {
		config.Reserved = parseReserved(reserved)
	}

	return config, nil
}

// parseReserved parses reserved bytes from various formats
// Supports: "1,2,3" or "AQID" (base64) or "010203" (hex)
func parseReserved(s string) []byte {
	// Try comma-separated decimal
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		result := make([]byte, len(parts))
		for i, p := range parts {
			v, _ := strconv.Atoi(strings.TrimSpace(p))
			result[i] = byte(v)
		}
		return result
	}

	// Try hex
	if decoded, err := hex.DecodeString(s); err == nil && len(decoded) > 0 {
		return decoded
	}

	// Try base64
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil && len(decoded) > 0 {
		return decoded
	}

	if decoded, err := base64.RawStdEncoding.DecodeString(s); err == nil && len(decoded) > 0 {
		return decoded
	}

	return nil
}

