package splithttp

import (
	"context"
	"net"

	xraynet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/splithttp"
	"github.com/xtls/xray-core/transport/internet/tls"

	// Register splithttp transport
	_ "github.com/xtls/xray-core/transport/internet/splithttp"
)

// Config holds the splithttp transport configuration
type Config struct {
	Host           string
	Path           string
	Headers        map[string]string
	ServerName     string
	TLS            bool
	SkipCertVerify bool
	Mode           string // stream-one, stream-up, packet-up
}

// Dial creates a new splithttp connection using xray-core implementation
func Dial(ctx context.Context, addr string, config *Config) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	port, err := xraynet.PortFromString(portStr)
	if err != nil {
		return nil, err
	}

	serverName := config.ServerName
	if serverName == "" {
		serverName = host
	}

	// Create splithttp config
	mode := config.Mode
	if mode == "" {
		mode = "stream-one"
	}

	splithttpConfig := &splithttp.Config{
		Host: serverName,
		Path: config.Path,
		Mode: mode,
	}

	// Create stream settings
	streamSettings := &internet.MemoryStreamConfig{
		ProtocolName:     "splithttp",
		ProtocolSettings: splithttpConfig,
	}

	// Add TLS if enabled
	if config.TLS {
		tlsConfig := &tls.Config{
			ServerName:    serverName,
			AllowInsecure: config.SkipCertVerify,
		}
		streamSettings.SecurityType = "tls"
		streamSettings.SecuritySettings = tlsConfig
	}

	// Create destination
	dest := xraynet.Destination{
		Network: xraynet.Network_TCP,
		Address: xraynet.DomainAddress(host),
		Port:    port,
	}

	// Dial using xray-core splithttp
	conn, err := splithttp.Dial(ctx, dest, streamSettings)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
