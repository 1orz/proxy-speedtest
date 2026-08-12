package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/1orz/proxy-speedtest/common/pool"
	"github.com/1orz/proxy-speedtest/internal/parser"
	"github.com/1orz/proxy-speedtest/internal/xray"
	"github.com/1orz/proxy-speedtest/stats"
)

const (
	// 微软 CDN 上的 dotnetfx35.exe(~231MB),仅作最终兜底,已验证可用。
	DownloadLinkDefault = "https://download.microsoft.com/download/2/0/E/20E90413-712F-438C-988E-FDAA79A8AC3D/dotnetfx35.exe"
	// Cloudflare 全球 Anycast。注意:__down?bytes= 的上限当前为 <1e8(>=1e8 直接 403),
	// 故取 10MB;文件偏小,建议配合多线程下载充分利用测试时长。
	CloudflareLink   = "https://speed.cloudflare.com/__down?bytes=10000000"
	HetznerDE1G      = "https://fsn1-speed.hetzner.com/1GB.bin"
	HetznerUS1G      = "https://ash-speed.hetzner.com/1GB.bin"
	LinodeJP100M     = "https://speedtest.tokyo2.linode.com/100MB-tokyo2.bin"
	VultrSG100M      = "https://sgp-ping.vultr.com/vultr.com.100MB.bin"
	OVH1G            = "https://proof.ovh.net/files/1Gb.dat"
	DataPacketUS100M = "http://lax.download.datapacket.com/100mb.bin"
	// 华为云镜像(~2.3GB Ubuntu ISO),国内有 CDN 节点、海外亦可达。
	HuaweiCN2G = "https://mirrors.huaweicloud.com/ubuntu-releases/bionic/ubuntu-18.04.6-desktop-amd64.iso"

	// 上传测速端点(POST 接收即丢弃 body)。可用的公共 sink 稀缺:
	// CloudflareUpLink 为 anycast 就近、无大小上限、纯丢弃(响应 0 字节),首选;
	// DLPTestUpLink 为美国固定、返回 {"ok":true} 小响应,备选。
	CloudflareUpLink = "https://speed.cloudflare.com/__up"
	DLPTestUpLink    = "https://dlptest.com/api/http-post/"

	// 自建 Cloudflare Worker 测速端点(需 X-Speedtest-Key 头鉴权)。URL 与 key 均可在前端改,
	// 这里只是预置默认值。下载默认 500MiB(worker 上限 1GiB),上传走 __up(受 CF 套餐请求体上限)。
	WorkerDownDefault = "https://cf-sp.orbitintel.com/__down?bytes=524288000"
	WorkerUpDefault   = "https://cf-sp.orbitintel.com/__up"
)

// 自建对象存储网关(S3 兼容)的上传端点。与丢弃型 sink 不同,它是真的把对象写进桶里:
//
//	PUT {base}/{bucket}/{key}
//	  x-upload-key:          鉴权(用户在「自定义请求头」里填,不预置在代码里)
//	  x-amz-content-sha256:  请求体的 sha256
//	  x-amz-acl:             bucket-owner-full-control
//	  Content-Type:          application/octet-stream
//
// URL 与其它预设一样可在前端改;测速会把同一个 key 反复覆盖写,建议指向一个专用的
// 临时路径并配上生命周期规则。
const (
	// UploadCDN 是该端点的预设 key,需与前端 UPLOAD_ENDPOINTS 保持一致。
	UploadCDN = "cdn"

	// UploadKeyHeader 是网关的鉴权头名。key 的值由用户自己填,这里只固定头名。
	UploadKeyHeader = "x-upload-key"

	// CDNUpDefault 是预置的默认上传地址(前端 CDN_UP_URL 与之保持一致)。
	CDNUpDefault = "https://oassets.orbitfin.ai/filing-reports/speedtest-probe.bin"

	uploadACL = "bucket-owner-full-control"
)

// UploadProfile 描述一个上传端点的调用约定。丢弃型 sink(Cloudflare __up 等)用
// POST 即可;S3 兼容网关要 PUT,并且要按对象存储的约定带上内容摘要与 ACL。
type UploadProfile struct {
	Method        string // POST / PUT
	ContentSHA256 bool   // 是否发 x-amz-content-sha256
	ACL           string // x-amz-acl;空则不发
	ChunkBytes    int64  // 单次请求的请求体大小;0 = uploadChunkDefault
}

// GetUploadProfile 按端点 key 返回调用约定。未知 key 一律按丢弃型 sink 处理。
func GetUploadProfile(size string) UploadProfile {
	if size == UploadCDN {
		return UploadProfile{Method: http.MethodPut, ContentSHA256: true, ACL: uploadACL}
	}
	return UploadProfile{Method: http.MethodPost}
}

// GetDownloadURL 把前端/命令行的端点 key 映射到具体下载 URL。key 需与前端
// DOWNLOAD_ENDPOINTS 保持一致;customURL 非空时优先使用。
func GetDownloadURL(size string, customURL string) string {
	if customURL != "" {
		return customURL
	}
	switch size {
	case "cloudflare":
		return CloudflareLink
	case "hetzner-de":
		return HetznerDE1G
	case "hetzner-us":
		return HetznerUS1G
	case "linode-jp":
		return LinodeJP100M
	case "vultr-sg":
		return VultrSG100M
	case "ovh-eu":
		return OVH1G
	case "datapacket-us":
		return DataPacketUS100M
	case "huawei-cn":
		return HuaweiCN2G
	case "worker":
		return WorkerDownDefault
	default:
		return CloudflareLink
	}
}

// GetUploadURL 把上传端点 key 映射到具体 URL。key 需与前端 UPLOAD_ENDPOINTS 一致;
// customURL 非空时优先使用。
func GetUploadURL(size string, customURL string) string {
	if customURL != "" {
		return customURL
	}
	switch size {
	case "cloudflare":
		return CloudflareUpLink
	case "dlptest":
		return DLPTestUpLink
	case "worker":
		return WorkerUpDefault
	case UploadCDN:
		return CDNUpDefault
	default:
		return CloudflareUpLink
	}
}

type DownloadOption struct {
	URL              string
	DownloadTimeout  time.Duration
	HandshakeTimeout time.Duration
	Ranges           []Range
	Headers          map[string]string // 额外请求头(如自建 Worker 的 X-Speedtest-Key 鉴权)
}

// applyHeaders 把非空的自定义请求头写入 req(用于鉴权等)。
func applyHeaders(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		if k != "" && v != "" {
			req.Header.Set(k, v)
		}
	}
}

type Discard struct {
	total stats.Counter
}

func (e *Discard) Write(p []byte) (n int, err error) {
	n = len(p)
	pool.Put(p)
	e.total.Add(int64(n))
	return n, nil
}

func (e *Discard) Size() int64 {
	return e.total.Set(0)
}

func ByteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B/s", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB/s",
		float64(b)/float64(div), "KMGTPE"[exp])
}

func ByteCountIECTrim(b int64) string {
	result := ByteCountIEC(b)
	return strings.TrimSuffix(result, "/s")
}

// createDialer creates an xray dialer from a proxy link
func createDialer(link string) (*xray.Dialer, error) {
	config, err := parser.ParseLink(link)
	if err != nil {
		return nil, err
	}
	return xray.NewDialer(config)
}

func Download(link string, timeout time.Duration, handshakeTimeout time.Duration, resultChan chan<- int64, startChan chan<- time.Time) (int64, error) {
	return DownloadWithURL(link, timeout, handshakeTimeout, resultChan, startChan, DownloadLinkDefault)
}

func DownloadWithURL(link string, timeout time.Duration, handshakeTimeout time.Duration, resultChan chan<- int64, startChan chan<- time.Time, downloadURL string) (int64, error) {
	ctx := context.Background()
	dialer, err := createDialer(link)
	if err != nil {
		return 0, err
	}
	defer dialer.Close()

	if downloadURL == "" {
		downloadURL = DownloadLinkDefault
	}
	option := DownloadOption{
		DownloadTimeout:  timeout,
		HandshakeTimeout: handshakeTimeout,
		URL:              downloadURL,
	}
	return downloadInternal(ctx, option, resultChan, startChan, dialer.DialContext)
}

// DownloadWithURLThreads 用 threads 条并行连接下载并聚合总吞吐。每条连接在超时前不断重新
// 请求同一 URL,因此即便是很小的文件也能持续占满整个测试时长;上报给 resultChan 的每秒样本
// 是所有线程之和,返回值为峰值每秒聚合速度。单线程也走同一 ticker,保证 CLI/WebSocket
// 每秒都能收到一次采样,并在有限大小文件结束后继续请求直到超时。
//
// ctx 为调用方上下文:客户端断开/用户点终止时取消它,本次下载(含全部 worker)会立即停止,
// 避免协程与代理连接泄漏。注意:threads 是"单个节点测速内部的并行连接数",与"并发数"(同时
// 测多少个节点)是两个不同维度。
func DownloadWithURLThreads(ctx context.Context, link string, timeout time.Duration, handshakeTimeout time.Duration, resultChan chan<- int64, startChan chan<- time.Time, downloadURL string, threads int, headers map[string]string) (int64, error) {
	dialer, err := createDialer(link)
	if err != nil {
		return 0, err
	}
	defer dialer.Close()
	if downloadURL == "" {
		downloadURL = DownloadLinkDefault
	}

	// 下载上下文派生自调用方 ctx,并叠加本次测速超时;调用方取消会一并终止下载。
	dlCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if threads < 1 {
		threads = 1
	}

	var counter atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			transport := &http.Transport{DialContext: dialer.DialContext}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport}
			buf := pool.Get(20 * 1024)
			defer pool.Put(buf)
			for dlCtx.Err() == nil {
				req, err := http.NewRequestWithContext(dlCtx, "GET", downloadURL, nil)
				if err != nil {
					return
				}
				applyHeaders(req, headers)
				resp, err := client.Do(req)
				if err != nil {
					// 连接/请求失败:短暂退避后重试,直到超时或被取消
					select {
					case <-dlCtx.Done():
						return
					case <-time.After(200 * time.Millisecond):
						continue
					}
				}
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					// 非 2xx(如小文件被 Cloudflare 高频重下限流返回的 403/429):
					// 不能把错误页字节计入吞吐,退避更久后再试,减轻对端限流压力。
					resp.Body.Close()
					select {
					case <-dlCtx.Done():
						return
					case <-time.After(500 * time.Millisecond):
						continue
					}
				}
				for dlCtx.Err() == nil {
					nr, er := resp.Body.Read(buf)
					if nr > 0 {
						counter.Add(int64(nr))
					}
					if er != nil {
						break
					}
				}
				resp.Body.Close()
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	start := time.Now()
	if startChan != nil {
		startChan <- start
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var max int64
	var total int64
	flush := func(reportZero bool) {
		v := counter.Swap(0)
		if v > 0 {
			total += v
			if v > max {
				max = v
			}
		}
		if resultChan != nil && (reportZero || v > 0) {
			// 发送也受 dlCtx 约束:调用方取消/读取方退出后不会永久阻塞
			select {
			case resultChan <- v:
			case <-dlCtx.Done():
			}
		}
	}
	for {
		select {
		case <-ticker.C:
			flush(true)
			// 所有线程持续失败(始终 0 字节)时,最多等 ~5s 即判失败,避免对死节点空转满时长
			if total == 0 && time.Since(start) >= 5*time.Second {
				return 0, nil
			}
		case <-done:
			flush(false)
			return max, nil
		case <-dlCtx.Done():
			flush(false)
			return max, nil
		}
	}
}

// zeroBlock 是上传 body 的只读零字节源(并发只读,安全);内容无所谓,测的是吞吐。
var zeroBlock = make([]byte, 32*1024)

// uploadChunkBytes 是单次 POST 上传的固定大小。用固定 Content-Length(而非 chunked)兼容
// CF __up 与 DLPTest;每块完成后仅在响应 2xx 时才把本次字节计入吞吐,避免坏端点/限流的失败
// 上传贡献虚假速度、并使死节点(total==0)判定失效(与下载先验状态码再计数对称)。
// uploadChunkDefault 是单次请求默认的请求体大小,可被 UploadProfile.ChunkBytes 覆盖。
//
// 4MiB 是这个测试里唯一真正的取舍点,别随手调大:
//   - 调大 → 请求数变少(对象存储按写操作计费时更省),但一块传不完整场测试的慢速链路
//     会一次都没成功 —— 只有 2xx 才计入吞吐,上传就直接显示 0。1MiB/s 的线路配 32MiB
//     的块,15 秒测试期内一块都发不完。
//   - 调小 → 每请求的固定开销(建连之外的 header/响应往返)占比升高,压低测得的吞吐。
//
// 4MiB 在 1MiB/s 的慢线上约 4 秒一块(测试时长内能成几块),在 500MiB/s 上约 8ms 一块
// (开销可忽略),两端都还站得住。
const uploadChunkDefault int64 = 4 * 1024 * 1024

// uploadDigestCache 缓存「size 个零字节」的 sha256,供 S3 兼容网关的
// x-amz-content-sha256 使用。请求体内容恒定,按大小算一次即可 —— 否则每块都要重算
// 一遍摘要,白烧 CPU 还压低吞吐。
var uploadDigestCache sync.Map // int64 -> string

func uploadBodyDigest(size int64) string {
	if v, ok := uploadDigestCache.Load(size); ok {
		return v.(string)
	}
	h := sha256.New()
	for remaining := size; remaining > 0; {
		n := int64(len(zeroBlock))
		if n > remaining {
			n = remaining
		}
		h.Write(zeroBlock[:n])
		remaining -= n
	}
	sum := hex.EncodeToString(h.Sum(nil))
	uploadDigestCache.Store(size, sum)
	return sum
}

// uploadBody 是单次上传请求的 body:被 http.Client 读取时吐出零字节并累加本次的 sent 计数,
// 发满 remaining 或 ctx 取消后返回 io.EOF。计数发生在 Read(交给 transport)时刻,受 TCP 流控
// 回压,约等于实际发送速率。
type uploadBody struct {
	ctx       context.Context
	sent      *atomic.Int64
	remaining int64
}

func (b *uploadBody) Read(p []byte) (int, error) {
	if b.ctx.Err() != nil {
		return 0, io.EOF
	}
	if b.remaining <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > b.remaining {
		n = int(b.remaining)
	}
	for i := 0; i < n; {
		i += copy(p[i:n], zeroBlock)
	}
	b.remaining -= int64(n)
	b.sent.Add(int64(n))
	return n, nil
}

// UploadWithURLThreads 对称于 DownloadWithURLThreads:threads 个并行 POST 把零字节流上传到
// uploadURL(接收即丢弃型端点,如 Cloudflare __up),atomic 聚合每秒吞吐,返回峰值每秒速度。
// ctx 取消会终止全部上传 worker。
func UploadWithURLThreads(ctx context.Context, link string, timeout time.Duration, handshakeTimeout time.Duration, resultChan chan<- int64, startChan chan<- time.Time, uploadURL string, threads int, headers map[string]string, profile UploadProfile) (int64, error) {
	if profile.Method == "" {
		profile.Method = http.MethodPost
	}
	if uploadURL == "" {
		// 只有丢弃型 sink 才有通用兜底;对象存储网关缺地址必须报错,不能悄悄换成
		// 公共 sink 让用户以为测的是自己的网关。
		if profile.Method != http.MethodPost {
			return 0, fmt.Errorf("upload endpoint is empty: fill in the upload URL for the %q preset", UploadCDN)
		}
		uploadURL = CloudflareUpLink
	}
	dialer, err := createDialer(link)
	if err != nil {
		return 0, err
	}
	defer dialer.Close()
	if threads < 1 {
		threads = 1
	}
	chunk := profile.ChunkBytes
	if chunk <= 0 {
		chunk = uploadChunkDefault
	}

	dlCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var counter atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			transport := &http.Transport{DialContext: dialer.DialContext}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport}
			for dlCtx.Err() == nil {
				var sent atomic.Int64
				req, err := http.NewRequestWithContext(dlCtx, profile.Method, uploadURL, &uploadBody{ctx: dlCtx, sent: &sent, remaining: chunk})
				if err != nil {
					return
				}
				req.ContentLength = chunk // 固定 Content-Length,避免 chunked 被部分端点拒绝
				req.Header.Set("Content-Type", "application/octet-stream")
				req.Header.Set("User-Agent", "Mozilla/5.0")
				// 对象存储网关按 S3 约定校验内容摘要并要求 ACL;丢弃型 sink 不需要。
				if profile.ContentSHA256 {
					req.Header.Set("x-amz-content-sha256", uploadBodyDigest(chunk))
				}
				if profile.ACL != "" {
					req.Header.Set("x-amz-acl", profile.ACL)
				}
				applyHeaders(req, headers)
				resp, err := client.Do(req)
				if err != nil {
					// 连接/发送失败(含 ctx 取消致 body 短于 Content-Length):本次不计入,退避重试
					select {
					case <-dlCtx.Done():
						return
					case <-time.After(200 * time.Millisecond):
						continue
					}
				}
				ok := resp.StatusCode >= 200 && resp.StatusCode < 300
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if ok {
					// 仅成功上传才计入吞吐;失败页/限流的字节不计,死端点 total 保持 0 可被判失败
					counter.Add(sent.Load())
				} else {
					select {
					case <-dlCtx.Done():
						return
					case <-time.After(500 * time.Millisecond):
						continue
					}
				}
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	start := time.Now()
	if startChan != nil {
		startChan <- start
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var max int64
	var total int64
	flush := func(reportZero bool) {
		v := counter.Swap(0)
		if v > 0 {
			total += v
			if v > max {
				max = v
			}
		}
		if resultChan != nil && (reportZero || v > 0) {
			select {
			case resultChan <- v:
			case <-dlCtx.Done():
			}
		}
	}
	for {
		select {
		case <-ticker.C:
			flush(true)
			// 所有线程持续失败(始终 0 字节)时最多等 ~5s 即判失败,避免对死节点空转满时长
			if total == 0 && time.Since(start) >= 5*time.Second {
				return 0, nil
			}
		case <-done:
			flush(false)
			return max, nil
		case <-dlCtx.Done():
			flush(false)
			return max, nil
		}
	}
}

func downloadInternal(ctx context.Context, option DownloadOption, resultChan chan<- int64, startOuterChan chan<- time.Time, dialContext func(ctx context.Context, network, addr string) (net.Conn, error)) (int64, error) {
	var max int64 = 0
	httpTransport := &http.Transport{}
	httpClient := &http.Client{Transport: httpTransport, Timeout: option.HandshakeTimeout}
	if dialContext != nil {
		httpTransport.DialContext = dialContext
	}
	req, err := http.NewRequestWithContext(ctx, "GET", option.URL, nil)
	if err != nil {
		return max, err
	}
	applyHeaders(req, option.Headers)
	response, err := httpClient.Do(req)
	if err != nil {
		return max, err
	}
	defer response.Body.Close()
	// 非 2xx 视为失败,避免把错误页字节计入下载速度。
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return max, nil
	}
	prev := time.Now()
	if startOuterChan != nil {
		startOuterChan <- prev
	}
	var total int64
	buf := pool.Get(20 * 1024)
	defer pool.Put(buf)
	for {
		nr, er := response.Body.Read(buf)
		total += int64(nr)
		now := time.Now()
		if now.Sub(prev) >= time.Second || er != nil {
			prev = now
			if max < total {
				max = total
			}
			if resultChan != nil {
				// 发送受 ctx 约束:调用方取消/读取方退出后不会永久阻塞
				select {
				case resultChan <- total:
				case <-ctx.Done():
					return max, nil
				}
			}
			total = 0
		}
		if er != nil {
			break
		}
	}
	return max, nil
}

func DownloadComplete(link string, timeout time.Duration, handshakeTimeout time.Duration) (int64, error) {
	ctx := context.Background()
	dialer, err := createDialer(link)
	if err != nil {
		return 0, err
	}
	defer dialer.Close()
	return downloadCompleteInternal(ctx, CloudflareLink, timeout, handshakeTimeout, dialer.DialContext)
}

func downloadCompleteInternal(ctx context.Context, url string, timeout time.Duration, handshakeTimeout time.Duration, dialContext func(ctx context.Context, network, addr string) (net.Conn, error)) (int64, error) {
	var max int64 = 0
	httpTransport := &http.Transport{}
	httpClient := &http.Client{Transport: httpTransport, Timeout: handshakeTimeout}
	if dialContext != nil {
		httpTransport.DialContext = dialContext
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return max, err
	}
	response, err := httpClient.Do(req)
	if err != nil {
		return max, err
	}
	defer response.Body.Close()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	var total int64
	buf := pool.Get(20 * 1024)
	defer pool.Put(buf)
	for ctx.Err() == nil {
		nr, er := response.Body.Read(buf)
		total += int64(nr)
		if er != nil {
			break
		}
	}

	elapsedMs := time.Since(start).Milliseconds()
	if elapsedMs < 1 {
		elapsedMs = 1
	}
	max = total * 1000 / elapsedMs
	return max, nil
}

func WSDownload(link string, timeout time.Duration, handshakeTimeout time.Duration, resultChan chan<- int64) (int64, error) {
	return 0, nil
}
