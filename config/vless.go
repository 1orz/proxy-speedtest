package config

import (
	"errors"

	"github.com/1orz/proxy-speedtest/outbound"
	"github.com/1orz/proxy-speedtest/proxy/xray"
)

// VlessLinkToXrayConfig parses a vless:// link to xray.ProxyConfig
func VlessLinkToXrayConfig(link string) (*xray.ProxyConfig, error) {
	return xray.ParseVLESSLink(link)
}

// VlessConfigFromMap creates ProxyConfig from a map (for clash config parsing)
func VlessConfigFromMap(mapping map[string]interface{}) (*xray.ProxyConfig, error) {
	config := &xray.ProxyConfig{
		Protocol: "vless",
		Network:  "tcp",
	}

	if name, ok := mapping["name"].(string); ok {
		config.Name = name
	}

	if server, ok := mapping["server"].(string); ok {
		config.Address = server
	} else {
		return nil, errors.New("vless server is required")
	}

	if port, ok := mapping["port"].(int); ok {
		config.Port = uint16(port)
	} else if portFloat, ok := mapping["port"].(float64); ok {
		config.Port = uint16(portFloat)
	} else {
		return nil, errors.New("vless port is required")
	}

	if uuid, ok := mapping["uuid"].(string); ok {
		config.UUID = uuid
	} else {
		return nil, errors.New("vless uuid is required")
	}

	if flow, ok := mapping["flow"].(string); ok {
		config.Flow = flow
	}

	if network, ok := mapping["network"].(string); ok {
		config.Network = network
	}

	if tls, ok := mapping["tls"].(bool); ok {
		config.TLS = tls
	}

	if skipCertVerify, ok := mapping["skip-cert-verify"].(bool); ok {
		config.SkipCertVerify = skipCertVerify
	}

	if serverName, ok := mapping["servername"].(string); ok {
		config.ServerName = serverName
	}

	if fingerprint, ok := mapping["client-fingerprint"].(string); ok {
		config.Fingerprint = fingerprint
	}

	// Parse reality-opts
	if realityOpts, ok := mapping["reality-opts"].(map[string]interface{}); ok {
		config.Reality = &xray.RealityConfig{}
		if publicKey, ok := realityOpts["public-key"].(string); ok {
			config.Reality.PublicKey = publicKey
		}
		if shortID, ok := realityOpts["short-id"].(string); ok {
			config.Reality.ShortID = shortID
		}
	}

	// Parse ws-opts
	if wsOpts, ok := mapping["ws-opts"].(map[string]interface{}); ok {
		if path, ok := wsOpts["path"].(string); ok {
			config.Path = path
		}
		if headers, ok := wsOpts["headers"].(map[string]interface{}); ok {
			config.Headers = make(map[string]string)
			for k, v := range headers {
				if vs, ok := v.(string); ok {
					config.Headers[k] = vs
					if k == "Host" || k == "host" {
						config.Host = vs
					}
				}
			}
		}
	}

	// Parse grpc-opts
	if grpcOpts, ok := mapping["grpc-opts"].(map[string]interface{}); ok {
		if serviceName, ok := grpcOpts["grpc-service-name"].(string); ok {
			config.ServiceName = serviceName
		}
	}

	return config, nil
}

func init() {
	// Register all protocol dialer creators
	outbound.RegisterDialerCreator("vless", func(link string) (outbound.Dialer, error) {
		config, err := xray.ParseVLESSLink(link)
		if err != nil {
			return nil, err
		}
		return outbound.NewXrayDialer(config)
	})

	outbound.RegisterDialerCreator("vmess", func(link string) (outbound.Dialer, error) {
		config, err := xray.ParseVMessLink(link)
		if err != nil {
			return nil, err
		}
		return outbound.NewXrayDialer(config)
	})

	outbound.RegisterDialerCreator("trojan", func(link string) (outbound.Dialer, error) {
		config, err := xray.ParseTrojanLink(link)
		if err != nil {
			return nil, err
		}
		return outbound.NewXrayDialer(config)
	})

	outbound.RegisterDialerCreator("ss", func(link string) (outbound.Dialer, error) {
		config, err := xray.ParseSSLink(link)
		if err != nil {
			return nil, err
		}
		return outbound.NewXrayDialer(config)
	})
}
