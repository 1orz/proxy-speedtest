package outbound

import (
	"context"
	"errors"
	"net"
	"strconv"

	C "github.com/1orz/proxy-speedtest/constant"
	"github.com/1orz/proxy-speedtest/proxy/xray"
	"github.com/1orz/proxy-speedtest/stats"
)

var ErrUDPNotSupported = errors.New("UDP not supported for this protocol")

// XrayDialer is a universal dialer using xray-core
type XrayDialer struct {
	*Base
	config *xray.ProxyConfig
}

// NewXrayDialer creates a new xray-based dialer
func NewXrayDialer(config *xray.ProxyConfig) (*XrayDialer, error) {
	addr := net.JoinHostPort(config.Address, strconv.Itoa(int(config.Port)))

	name := config.Name
	if name == "" {
		name = config.Protocol
	}

	return &XrayDialer{
		Base: &Base{
			name: name,
			addr: addr,
			udp:  false,
		},
		config: config,
	}, nil
}

// DialContext implements Dialer interface
func (d *XrayDialer) DialContext(ctx context.Context, metadata *C.Metadata) (net.Conn, error) {
	targetPort := 80
	if metadata.DstPort != "" {
		port, err := strconv.Atoi(metadata.DstPort)
		if err == nil {
			targetPort = port
		}
	}

	conn, err := xray.Dial(ctx, d.config, metadata.Host, targetPort)
	if err != nil {
		return nil, err
	}

	return stats.NewStatsConn(conn), nil
}

// DialUDP implements Dialer interface (not fully supported yet)
func (d *XrayDialer) DialUDP(metadata *C.Metadata) (net.PacketConn, error) {
	return nil, ErrUDPNotSupported
}

// Close closes the underlying xray instance
func (d *XrayDialer) Close() error {
	xray.CloseInstance(d.config)
	return nil
}

// Config returns the proxy config
func (d *XrayDialer) Config() *xray.ProxyConfig {
	return d.config
}

