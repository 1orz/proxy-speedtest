package config

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/xxf098/lite-proxy/outbound"
)

// VlessLinkToVlessOption parses a vless:// link to VlessOption
// Format: vless://uuid@host:port?type=tcp&security=reality&pbk=xxx&fp=chrome&sni=xxx&flow=xtls-rprx-vision#remarks
func VlessLinkToVlessOption(link string) (*outbound.VlessOption, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "vless" {
		return nil, errors.New("not a vless link")
	}

	uuid := u.User.Username()
	if uuid == "" {
		return nil, errors.New("vless uuid is empty")
	}

	hostport := u.Host
	host, portStr, err := net.SplitHostPort(hostport)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	option := &outbound.VlessOption{
		Name:    "vless",
		Server:  host,
		Port:    uint16(port),
		UUID:    uuid,
		Network: "tcp",
		TLS:     false,
		Remarks: u.Fragment,
	}

	// Parse query parameters
	query := u.Query()

	// Network/type
	if t := query.Get("type"); t != "" {
		option.Network = t
	}

	// Flow (xtls-rprx-vision)
	if flow := query.Get("flow"); flow != "" {
		option.Flow = flow
	}

	// Security
	security := query.Get("security")
	switch security {
	case "tls":
		option.TLS = true
	case "reality":
		option.TLS = true
		// Parse Reality options
		pbk := query.Get("pbk")
		if pbk == "" {
			return nil, errors.New("reality public key (pbk) is required")
		}
		option.Reality = &outbound.RealityOptions{
			PublicKey: pbk,
			ShortID:   query.Get("sid"),
		}
	case "none", "":
		option.TLS = false
	default:
		// Unknown security type, treat as TLS
		option.TLS = true
	}

	// SNI
	if sni := query.Get("sni"); sni != "" {
		option.ServerName = sni
	}

	// Fingerprint
	if fp := query.Get("fp"); fp != "" {
		option.Fingerprint = fp
	}

	// Skip cert verify
	if insecure := query.Get("allowInsecure"); insecure == "1" || insecure == "true" {
		option.SkipCertVerify = true
	}

	// ALPN
	if alpn := query.Get("alpn"); alpn != "" {
		option.ALPN = strings.Split(alpn, ",")
	}

	// WebSocket options
	if option.Network == "ws" {
		path := query.Get("path")
		if path == "" {
			path = "/"
		}
		option.WSPath = path
		option.WSOpts = outbound.WSOptions{
			Path: path,
		}

		if host := query.Get("host"); host != "" {
			option.WSHeaders = map[string]string{
				"Host": host,
			}
			option.WSOpts.Headers = map[string]string{
				"Host": host,
			}
		}
	}

	// HTTP/2 options
	if option.Network == "h2" {
		path := query.Get("path")
		if path == "" {
			path = "/"
		}
		host := query.Get("host")
		if host == "" {
			host = option.Server
		}
		option.HTTP2Opts = outbound.HTTP2Options{
			Host: []string{host},
			Path: path,
		}
		// H2 requires TLS
		option.TLS = true
	}

	// gRPC options
	if option.Network == "grpc" {
		serviceName := query.Get("serviceName")
		if serviceName == "" {
			serviceName = query.Get("service-name")
		}
		option.GrpcOpts = outbound.GrpcOptions{
			GrpcServiceName: serviceName,
		}
	}

	// xhttp/splithttp options (treated similar to ws for now)
	if option.Network == "xhttp" || option.Network == "splithttp" {
		path := query.Get("path")
		if path == "" {
			path = "/"
		}
		option.WSPath = path
		option.WSOpts = outbound.WSOptions{
			Path: path,
		}
		if host := query.Get("host"); host != "" {
			option.WSHeaders = map[string]string{
				"Host": host,
			}
		}
	}

	// Set remarks from fragment
	if option.Remarks == "" {
		option.Remarks = option.Server
	}

	return option, nil
}

// VlessConfigFromMap creates VlessOption from a map (for clash config parsing)
func VlessConfigFromMap(mapping map[string]interface{}) (*outbound.VlessOption, error) {
	option := &outbound.VlessOption{
		Name:    "vless",
		Network: "tcp",
	}

	if name, ok := mapping["name"].(string); ok {
		option.Name = name
	}

	if server, ok := mapping["server"].(string); ok {
		option.Server = server
	} else {
		return nil, errors.New("vless server is required")
	}

	if port, ok := mapping["port"].(int); ok {
		option.Port = uint16(port)
	} else if portFloat, ok := mapping["port"].(float64); ok {
		option.Port = uint16(portFloat)
	} else {
		return nil, errors.New("vless port is required")
	}

	if uuid, ok := mapping["uuid"].(string); ok {
		option.UUID = uuid
	} else {
		return nil, errors.New("vless uuid is required")
	}

	if flow, ok := mapping["flow"].(string); ok {
		option.Flow = flow
	}

	if network, ok := mapping["network"].(string); ok {
		option.Network = network
	}

	if tls, ok := mapping["tls"].(bool); ok {
		option.TLS = tls
	}

	if skipCertVerify, ok := mapping["skip-cert-verify"].(bool); ok {
		option.SkipCertVerify = skipCertVerify
	}

	if serverName, ok := mapping["servername"].(string); ok {
		option.ServerName = serverName
	}

	if fingerprint, ok := mapping["client-fingerprint"].(string); ok {
		option.Fingerprint = fingerprint
	}

	// Parse reality-opts
	if realityOpts, ok := mapping["reality-opts"].(map[string]interface{}); ok {
		option.Reality = &outbound.RealityOptions{}
		if publicKey, ok := realityOpts["public-key"].(string); ok {
			option.Reality.PublicKey = publicKey
		}
		if shortID, ok := realityOpts["short-id"].(string); ok {
			option.Reality.ShortID = shortID
		}
	}

	// Parse ws-opts
	if wsOpts, ok := mapping["ws-opts"].(map[string]interface{}); ok {
		if path, ok := wsOpts["path"].(string); ok {
			option.WSPath = path
			option.WSOpts.Path = path
		}
		if headers, ok := wsOpts["headers"].(map[string]interface{}); ok {
			option.WSHeaders = make(map[string]string)
			option.WSOpts.Headers = make(map[string]string)
			for k, v := range headers {
				if vs, ok := v.(string); ok {
					option.WSHeaders[k] = vs
					option.WSOpts.Headers[k] = vs
				}
			}
		}
	}

	// Parse grpc-opts
	if grpcOpts, ok := mapping["grpc-opts"].(map[string]interface{}); ok {
		if serviceName, ok := grpcOpts["grpc-service-name"].(string); ok {
			option.GrpcOpts.GrpcServiceName = serviceName
		}
	}

	return option, nil
}

func init() {
	outbound.RegisterDialerCreator("vless", func(link string) (outbound.Dialer, error) {
		vlessOption, err := VlessLinkToVlessOption(link)
		if err != nil {
			return nil, err
		}
		return outbound.NewVless(vlessOption)
	})
}

