package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/1orz/proxy-speedtest/config"
	_ "github.com/1orz/proxy-speedtest/config"
	"github.com/1orz/proxy-speedtest/dns"
	"github.com/1orz/proxy-speedtest/outbound"
	"github.com/1orz/proxy-speedtest/proxy"
	"github.com/1orz/proxy-speedtest/request"
	"github.com/1orz/proxy-speedtest/transport/resolver"
	"github.com/1orz/proxy-speedtest/tunnel"
	"github.com/1orz/proxy-speedtest/tunnel/adapter"
	"github.com/1orz/proxy-speedtest/tunnel/http"
	"github.com/1orz/proxy-speedtest/tunnel/socks"
	"github.com/1orz/proxy-speedtest/utils"
)

func StartInstance(c Config) (*proxy.Proxy, error) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "LocalHost", c.LocalHost)
	ctx = context.WithValue(ctx, "LocalPort", c.LocalPort)
	adapterServer, err := adapter.NewServer(ctx, nil)
	if err != nil {
		return nil, err
	}
	httpServer, err := http.NewServer(ctx, adapterServer)
	if err != nil {
		return nil, err
	}
	socksServer, err := socks.NewServer(ctx, adapterServer)
	if err != nil {
		return nil, err
	}
	sources := []tunnel.Server{httpServer, socksServer}
	sink, err := createSink(ctx, c.Link)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func(link string) {
		if c.Ping < 1 {
			return
		}
		if cfg, err := config.Link2Config(c.Link); err == nil {
			opt := request.PingOption{
				Attempts: c.Ping,
				TimeOut:  1200 * time.Millisecond,
			}
			info := fmt.Sprintf("%s %s", cfg.Remarks, net.JoinHostPort(cfg.Server, strconv.Itoa(cfg.Port)))
			if elapse, err := request.PingLinkInternal(link, opt); err == nil {
				info = fmt.Sprintf("%s \033[32m%dms\033[0m", info, elapse)
			} else {
				info = fmt.Sprintf("\033[31m%s\033[0m", err.Error())
			}
			log.Print(info)
		}
	}(c.Link)
	setDefaultResolver()
	p := proxy.NewProxy(ctx, cancel, sources, sink)
	return p, nil
}

func createSink(ctx context.Context, link string) (tunnel.Client, error) {
	var d outbound.Dialer
	matches, err := utils.CheckLink(link)
	if err != nil {
		return nil, err
	}
	creator, err := outbound.GetDialerCreator(matches[1])
	if err != nil {
		return nil, err
	}
	d, err = creator(link)
	if err != nil {
		return nil, err
	}
	if d != nil {
		return proxy.NewClient(ctx, d), nil
	}

	return nil, errors.New("not supported link")
}

func setDefaultResolver() {
	resolver.DefaultResolver = dns.DefaultResolver()
}
