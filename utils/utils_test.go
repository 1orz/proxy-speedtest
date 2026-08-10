package utils

import "testing"

func TestCheckLink(t *testing.T) {
	valid := []string{
		"vmess://abc",
		"trojan://user@example.com:443",
		"ss://abc#name",
		// direct:// 的 :// 之后可以什么都没有(仅可选的 #备注)。
		"direct://",
		"DIRECT://",
		"direct://#家宽",
	}
	for _, link := range valid {
		if _, err := CheckLink(link); err != nil {
			t.Fatalf("CheckLink(%q) = %v, want nil", link, err)
		}
	}

	invalid := []string{"", "direct", "vmess://", "ftp://example.com", "example.com"}
	for _, link := range invalid {
		if _, err := CheckLink(link); err == nil {
			t.Fatalf("CheckLink(%q) = nil, want error", link)
		}
	}
}
