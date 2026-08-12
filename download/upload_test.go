package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// 自建对象存储网关(S3 兼容)的上传约定必须一字不差,否则网关会整场 4xx:
// PUT + 正确的 x-amz-content-sha256 + x-amz-acl,用户自定义头照常透传。
// 尤其是摘要——算错的话真实网关每一块都拒收,而本地看到的只是一个 N/A。
func TestUploadWithURLThreadsPutProfile(t *testing.T) {
	var mu sync.Mutex
	var (
		requests   int
		methods    = map[string]int{}
		digestOK   = true
		aclSeen    string
		keySeen    string
		typeSeen   string
		lengthSeen int64
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sum := sha256.Sum256(body)
		mu.Lock()
		requests++
		methods[r.Method]++
		if hex.EncodeToString(sum[:]) != r.Header.Get("x-amz-content-sha256") {
			digestOK = false
		}
		aclSeen = r.Header.Get("x-amz-acl")
		keySeen = r.Header.Get(UploadKeyHeader)
		typeSeen = r.Header.Get("Content-Type")
		lengthSeen = r.ContentLength
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	headers := map[string]string{UploadKeyHeader: "secret-key"}
	_, err := UploadWithURLThreads(ctx, "direct://", 2*time.Second, 2*time.Second,
		nil, nil, srv.URL+"/bucket/speedtest/probe.bin", 1, headers, GetUploadProfile(UploadCDN))
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if requests == 0 {
		t.Fatal("no upload request reached the endpoint")
	}
	if methods[http.MethodPut] != requests {
		t.Fatalf("methods = %v, want all PUT", methods)
	}
	if !digestOK {
		t.Fatal("x-amz-content-sha256 does not match the body the gateway received")
	}
	if aclSeen != "bucket-owner-full-control" {
		t.Fatalf("x-amz-acl = %q", aclSeen)
	}
	if keySeen != "secret-key" {
		t.Fatalf("%s = %q, want the caller-supplied key", UploadKeyHeader, keySeen)
	}
	if typeSeen != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", typeSeen)
	}
	// 固定 Content-Length(而非 chunked):部分网关拒收 chunked PUT。
	if lengthSeen != uploadChunkDefault {
		t.Fatalf("Content-Length = %d, want %d", lengthSeen, uploadChunkDefault)
	}
}

// 丢弃型 sink 仍应是 POST,且不带对象存储那几个头 —— 别把新约定漏给了老端点。
func TestUploadWithURLThreadsDefaultProfileStaysPost(t *testing.T) {
	var mu sync.Mutex
	var method, acl, digest string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		mu.Lock()
		method = r.Method
		acl = r.Header.Get("x-amz-acl")
		digest = r.Header.Get("x-amz-content-sha256")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := UploadWithURLThreads(ctx, "direct://", 2*time.Second, 2*time.Second,
		nil, nil, srv.URL, 1, nil, GetUploadProfile("cloudflare")); err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if method != http.MethodPost {
		t.Fatalf("method = %q, want POST", method)
	}
	if acl != "" || digest != "" {
		t.Fatalf("discard sink got object-store headers: acl=%q digest=%q", acl, digest)
	}
}

// 用户把对象存储网关的地址清空时必须报错,绝不能悄悄回落到公共 sink
// (否则用户以为在测自建网关,其实测的是 Cloudflare)。
func TestUploadPutProfileRequiresURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := UploadWithURLThreads(ctx, "direct://", time.Second, time.Second,
		nil, nil, "", 1, nil, GetUploadProfile(UploadCDN))
	if err == nil {
		t.Fatal("empty PUT endpoint accepted, want an error")
	}
	if !strings.Contains(err.Error(), UploadCDN) {
		t.Fatalf("error should name the %q preset, got: %v", UploadCDN, err)
	}
}

// 预设 key 必须映射到内置默认地址(前端 CDN_UP_URL 与之对应),用户填了则以用户的为准。
func TestGetUploadURLCDNPreset(t *testing.T) {
	if got := GetUploadURL(UploadCDN, ""); got != CDNUpDefault {
		t.Fatalf("GetUploadURL(%q, \"\") = %q, want %q", UploadCDN, got, CDNUpDefault)
	}
	const custom = "https://gateway.example.com/bucket/probe.bin"
	if got := GetUploadURL(UploadCDN, custom); got != custom {
		t.Fatalf("custom URL should win, got %q", got)
	}
}

// 摘要是对固定零字节请求体算的,必须与实际发出的内容一致(且只算一次)。
func TestUploadBodyDigestMatchesBody(t *testing.T) {
	want := sha256.Sum256(make([]byte, uploadChunkDefault))
	if got := uploadBodyDigest(uploadChunkDefault); got != hex.EncodeToString(want[:]) {
		t.Fatalf("uploadBodyDigest = %s, want %s", got, hex.EncodeToString(want[:]))
	}
	if uploadBodyDigest(uploadChunkDefault) != uploadBodyDigest(uploadChunkDefault) {
		t.Fatal("digest is not stable")
	}
}
