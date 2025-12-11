package socks

import (
	"context"

	"github.com/1orz/proxy-speedtest/tunnel"
)

const Name = "SOCKS"

type Tunnel struct{}

func (t *Tunnel) Name() string {
	return Name
}

func (t *Tunnel) NewServer(ctx context.Context, server tunnel.Server) (tunnel.Server, error) {
	return NewServer(ctx, server)
}
