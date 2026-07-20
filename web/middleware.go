package web

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// statusRecorder 包装 http.ResponseWriter 以记录状态码与响应字节数,并透传 Hijack。
// gorilla WebSocket 升级依赖 http.Hijacker,若不透传,/test 的升级会因
// "response does not implement http.Hijacker" 而失败。
type statusRecorder struct {
	http.ResponseWriter
	status   int
	bytes    int
	hijacked bool
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not implement http.Hijacker")
	}
	conn, rw, err := hj.Hijack()
	if err == nil {
		r.hijacked = true
	}
	return conn, rw, err
}

// accessLog 记录每个 HTTP 请求(方法/路径/状态/字节/耗时/来源)。
// WebSocket(/test)升级后由 gorilla 直接向底层连接写 101、不经过 ResponseWriter,
// 因此这里按 Upgrade 头把状态补记为 101。
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// 提前捕获:serverFile 等 handler 会原地改写 r.URL.Path。
		method, path := r.Method, r.URL.RequestURI()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		status := rec.status
		if status == 0 {
			if rec.hijacked {
				// 连接被 Hijack(WebSocket 升级),gorilla 直接向底层 conn 写 101。
				status = http.StatusSwitchingProtocols // 101
			} else {
				status = http.StatusOK
			}
		}
		slog.Info("access",
			"remote", r.RemoteAddr,
			"method", method,
			"path", path,
			"status", status,
			"bytes", rec.bytes,
			"dur", time.Since(start).String(),
			"ua", r.UserAgent(),
		)
	})
}
