package main

import "testing"

func TestHeaderFlags(t *testing.T) {
	var headers headerFlags
	if err := headers.Set("Authorization: Bearer abc:def"); err != nil {
		t.Fatal(err)
	}
	if err := headers.Set(" X-Speedtest-Key : secret "); err != nil {
		t.Fatal(err)
	}
	if len(headers) != 2 {
		t.Fatalf("len=%d want 2", len(headers))
	}
	if headers[0].Name != "Authorization" || headers[0].Value != "Bearer abc:def" {
		t.Fatalf("unexpected first header: %+v", headers[0])
	}
	if headers[1].Name != "X-Speedtest-Key" || headers[1].Value != "secret" {
		t.Fatalf("unexpected second header: %+v", headers[1])
	}
}

func TestHeaderFlagsRejectInvalidValue(t *testing.T) {
	for _, value := range []string{"Authorization", ": value"} {
		var headers headerFlags
		if err := headers.Set(value); err == nil {
			t.Fatalf("Set(%q) succeeded, want error", value)
		}
	}
}

func TestOptionalBool(t *testing.T) {
	var value optionalBool
	if value.set {
		t.Fatal("zero value should not be marked as set")
	}
	if err := value.Set("true"); err != nil {
		t.Fatal(err)
	}
	if !value.set || !value.value {
		t.Fatalf("unexpected value: %+v", value)
	}
	if err := value.Set("false"); err != nil {
		t.Fatal(err)
	}
	if !value.set || value.value {
		t.Fatalf("unexpected value after false: %+v", value)
	}
}
