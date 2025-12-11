package config

import (
	"strings"

	"github.com/1orz/proxy-speedtest/common"
	"github.com/1orz/proxy-speedtest/outbound"
	"github.com/1orz/proxy-speedtest/proxy/xray"
	"github.com/1orz/proxy-speedtest/utils"
)

func Link2Dialer(link string) (outbound.Dialer, error) {
	matches, err := utils.CheckLink(link)
	if err != nil {
		return nil, err
	}
	var d outbound.Dialer

	switch strings.ToLower(matches[1]) {
	case "vmess":
		// Use original VMess implementation
		option, err := VmessLinkToVmessOption(link)
		if err != nil {
			return nil, err
		}
		d, err = outbound.NewVmess(option)
		if err != nil {
			return nil, err
		}
	case "vless":
		config, err := xray.ParseVLESSLink(link)
		if err != nil {
			return nil, err
		}
		d, err = outbound.NewXrayDialer(config)
		if err != nil {
			return nil, err
		}
	case "trojan":
		config, err := xray.ParseTrojanLink(link)
		if err != nil {
			return nil, err
		}
		d, err = outbound.NewXrayDialer(config)
		if err != nil {
			return nil, err
		}
	case "ss":
		config, err := xray.ParseSSLink(link)
		if err != nil {
			return nil, err
		}
		d, err = outbound.NewXrayDialer(config)
		if err != nil {
			return nil, err
		}
	default:
		return nil, common.NewError("Not Supported Link: " + matches[1])
	}

	return d, nil
}

type Config struct {
	Protocol string
	Remarks  string
	Server   string
	Net      string // vmess net type
	Port     int
	Password string
	SNI      string
}

func Link2Config(link string) (*Config, error) {
	matches, err := utils.CheckLink(link)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(matches[1]) {
	case "vmess":
		cfgVmess, err := VmessLinkToVmessConfigIP(link, false)
		if err != nil {
			return nil, err
		}
		remarks := cfgVmess.Ps
		if len(remarks) < 1 {
			remarks = cfgVmess.Add
		}
		return &Config{
			Protocol: "vmess",
			Remarks:  remarks,
			Server:   cfgVmess.Add,
			Port:     cfgVmess.PortInt,
			Net:      cfgVmess.Net,
			Password: cfgVmess.ID,
			SNI:      cfgVmess.Host,
		}, nil

	case "vless":
		config, err := xray.ParseVLESSLink(link)
		if err != nil {
			return nil, err
		}
		remarks := config.Name
		if remarks == "" {
			remarks = config.Address
		}
		return &Config{
			Protocol: "vless",
			Remarks:  remarks,
			Server:   config.Address,
			Port:     int(config.Port),
			Net:      config.Network,
			Password: config.UUID,
			SNI:      config.ServerName,
		}, nil

	case "trojan":
		config, err := xray.ParseTrojanLink(link)
		if err != nil {
			return nil, err
		}
		return &Config{
			Protocol: "trojan",
			Remarks:  config.Name,
			Server:   config.Address,
			Port:     int(config.Port),
			Net:      config.Network,
			Password: config.Password,
			SNI:      config.ServerName,
		}, nil

	case "ss":
		config, err := xray.ParseSSLink(link)
		if err != nil {
			return nil, err
		}
		return &Config{
			Protocol: "ss",
			Remarks:  config.Name,
			Server:   config.Address,
			Port:     int(config.Port),
			Password: config.Password,
		}, nil

	default:
		return nil, common.NewError("Not Supported Link: " + matches[1])
	}
}
