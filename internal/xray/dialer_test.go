package xray

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xtls/xray-core/core"
)

// TestBuildConfigDropsAllowInsecure 验证:对订阅标注了 skip-cert-verify / allowInsecure 的节点,
// 生成的 xray JSON 配置不再包含 "allowInsecure" 字段,因而不会被 xray-core v26+ 的
// core.LoadConfig 以 "removed feature" 拒绝(该特性在 2026-06-01 后被上游强制移除)。
func TestBuildConfigDropsAllowInsecure(t *testing.T) {
	cfg := &ProxyConfig{
		Protocol: ProtocolVLESS,
		Address:  "example.com",
		Port:     443,
		UUID:     "b831381d-6324-4d53-ad4f-8cda48b30811",
		Stream: &StreamSettings{
			Network:  "tcp",
			Security: "tls",
			TLS: &TLSSettings{
				ServerName:    "example.com",
				AllowInsecure: true, // 订阅标注了跳过证书校验
			},
		},
	}

	jsonConfig, err := buildJSONConfig(cfg)
	if err != nil {
		t.Fatalf("buildJSONConfig 失败: %v", err)
	}

	if strings.Contains(string(jsonConfig), "allowInsecure") {
		t.Fatalf("生成的配置仍包含 allowInsecure,会被 xray-core v26+ 拒绝:\n%s", jsonConfig)
	}

	// core.LoadConfig 应能成功解析该配置(不再返回 removed-feature 错误)。
	if _, err := core.LoadConfig("json", bytes.NewReader(jsonConfig)); err != nil {
		t.Fatalf("core.LoadConfig 拒绝了配置(allowInsecure 移除后不应发生): %v", err)
	}
}

// TestBuildConfigTrojanTLS 覆盖 Trojan + TLS + AllowInsecure 组合,确保同样不受影响。
func TestBuildConfigTrojanTLS(t *testing.T) {
	cfg := &ProxyConfig{
		Protocol: ProtocolTrojan,
		Address:  "example.com",
		Port:     443,
		Password: "hunter2",
		Stream: &StreamSettings{
			Network:  "tcp",
			Security: "tls",
			TLS: &TLSSettings{
				ServerName:    "example.com",
				AllowInsecure: true,
			},
		},
	}

	jsonConfig, err := buildJSONConfig(cfg)
	if err != nil {
		t.Fatalf("buildJSONConfig 失败: %v", err)
	}
	if strings.Contains(string(jsonConfig), "allowInsecure") {
		t.Fatalf("Trojan 配置仍包含 allowInsecure:\n%s", jsonConfig)
	}
	if _, err := core.LoadConfig("json", bytes.NewReader(jsonConfig)); err != nil {
		t.Fatalf("core.LoadConfig 拒绝了 Trojan 配置: %v", err)
	}
}

// TestBuildConfigServerNamePreserved 确保移除 allowInsecure 后 serverName 仍被写出,
// 使证书有效的节点(即便订阅标注了 skip-cert-verify)能走标准 TLS 校验并被测试。
func TestBuildConfigServerNamePreserved(t *testing.T) {
	cfg := &ProxyConfig{
		Protocol: ProtocolVLESS,
		Address:  "1.2.3.4",
		Port:     443,
		UUID:     "b831381d-6324-4d53-ad4f-8cda48b30811",
		Stream: &StreamSettings{
			Network:  "tcp",
			Security: "tls",
			TLS: &TLSSettings{
				ServerName:    "real.example.com",
				AllowInsecure: true,
			},
		},
	}

	jsonConfig, err := buildJSONConfig(cfg)
	if err != nil {
		t.Fatalf("buildJSONConfig 失败: %v", err)
	}
	if !strings.Contains(string(jsonConfig), "real.example.com") {
		t.Fatalf("生成的配置缺少 serverName,标准校验无法进行:\n%s", jsonConfig)
	}
}
