# LiteSpeedTest

LiteSpeedTest is a simple tool for batch testing proxy servers.

## Features

- Support VMess / VLESS / Trojan / Shadowsocks protocols
- Support subscription URLs and profile links
- Support Clash configuration files
- Support multiple transport protocols: TCP, WebSocket, gRPC, SplitHTTP/XHTTP, HTTP Upgrade
- Support TLS and Reality security
- Works without any subscription too: leave it empty (or pass `-direct`) for a plain
  download/upload bandwidth test of the machine itself

![build](https://github.com/1orz/proxy-speedtest/actions/workflows/ci.yaml/badge.svg?branch=master&event=push)

## Usage

### Web mode vs CLI mode

- **No subscription/link argument → web mode.** Starts the web UI (default `http://127.0.0.1:10888/`).
  Per-second download/upload samples are sent to both the browser and the server's stderr.
- **A subscription link, a share link (`vmess://` / `vless://` / `trojan://` / `ss://` …), a local
  file, or `-direct` → CLI mode.** Runs the speed test directly and prints/saves the result. Live
  progress goes to **stderr** with one download/upload sample per second; the result data goes to
  **stdout** (so it pipes cleanly).

```bash
# web mode: open http://127.0.0.1:10888/ in your browser
./proxy-speedtest
./proxy-speedtest -p 10889            # another port

# CLI mode: subscription (JSON to stdout by default)
./proxy-speedtest --test https://example.com/subscription

# direct mode: no subscription at all — just this machine's own download/upload speed
./proxy-speedtest -direct
./proxy-speedtest -direct --upload --threads 4 --output table

# CLI mode: a single share link, directly (flags may go before or after the link)
./proxy-speedtest "vmess://..."
./proxy-speedtest -o csv "vmess://..."

# CSV to a file (or pipe to stdout)
./proxy-speedtest --test https://sub --output csv --output-file result.csv
./proxy-speedtest --test https://sub -o csv > result.csv

# human-readable table in the terminal
./proxy-speedtest --test https://sub --output table

# produce JSON + a result image in one run
./proxy-speedtest --test https://sub --output json,pic --output-pic result.png

# custom timeout (seconds) and concurrency
./proxy-speedtest --test https://sub --timeout 20 --concurrency 4

# multi-threaded download per node
./proxy-speedtest --test https://sub --threads 4

# download endpoint preset (cloudflare, hetzner-de, hetzner-us, linode-jp, vultr-sg, ovh-eu, datapacket-us, huawei-cn, worker)
./proxy-speedtest --test https://sub --download-size cloudflare

# custom download URL
./proxy-speedtest --test https://sub --download-url "https://speed.cloudflare.com/__down?bytes=50000000"

# download + upload with custom endpoints and request headers
./proxy-speedtest --test https://sub --upload \
  --download-url "https://speed.example.com/__down?bytes=50000000" \
  --upload-url "https://speed.example.com/__up" \
  -H "X-Speedtest-Key: shared-secret" \
  -dH "Accept-Encoding: identity" \
  -uH "Authorization: Bearer upload-token"

# use a config file
./proxy-speedtest --config config.json --test https://sub
```

### Direct mode (no subscription)

A subscription is optional. With none, the tool degrades into a plain bandwidth tester for the
machine it runs on — no proxy anywhere in the path. Only the **download endpoint is required**
(a preset is selected by default, so it always has a value); **upload** and **request headers**
stay optional.

- **Web UI:** just leave the subscription list empty and press *Start Direct Test*. Uploading a
  config file or filling in any subscription switches back to proxy testing.
- **CLI:** `-direct`. It is ignored when `-test` or a positional share link is given.

```bash
./proxy-speedtest -direct                                     # download only, Cloudflare preset
./proxy-speedtest -direct --upload --output table             # download + upload
./proxy-speedtest -direct --download-size hetzner-de --threads 8
./proxy-speedtest -direct \
  --download-url "https://speed.example.com/__down?bytes=524288000" \
  --upload --upload-url "https://speed.example.com/__up" \
  -H "X-Speedtest-Key: shared-secret"
```

Internally this is one synthetic node, `direct://` (accepted anywhere a share link is — including
the subscription box, so a direct row can be mixed into a normal run as a baseline). Two behaviours
differ from proxy nodes:

- **Ping** measures the HTTP round trip to the *download endpoint* rather than probing
  `clients3.google.com/generate_204` — there is no proxy reachability to establish, and that probe
  is unreachable on plenty of networks.
- A failed ping does **not** cancel the speed test: bandwidth is the point of the run, and some
  endpoints refuse probe requests while serving downloads perfectly.

`direct://` rows are excluded from the `text` output and from *Copy working nodes*, since they are
not importable proxies.

### Upload endpoints

Public "discard sinks" that accept a POST and throw the body away are scarce, so upload testing
also supports a **self-hosted S3-compatible object-storage gateway** (`-upload-size cdn`). Unlike
a discard sink it really writes the object, using the gateway contract:

```
PUT {base}/{bucket}/{key}
  x-upload-key:         <auth>          ← you supply this as a request header
  x-amz-content-sha256: <sha256 of the body>
  x-amz-acl:            bucket-owner-full-control
  Content-Type:         application/octet-stream
```

The preset ships with a default URL (`https://oassets.orbitfin.ai/filing-reports/speedtest-probe.bin`)
that is editable like any other endpoint (web UI field, or `-upload-url`). The auth key is **not**
baked into the frontend — pass it as a custom request header, exactly like the Cloudflare Worker
preset's `X-Speedtest-Key`:

- **Web UI:** pick *Object storage gateway*, adjust the URL if needed, then add
  `x-upload-key` under *Custom Request Headers* — the quick-add buttons there insert the header
  name for you, you only type the value. (Untick "same headers for download & upload" if the
  download endpoint shouldn't get it.)
- **CLI:** `-uH "x-upload-key: …"` (upload only) or `-H` for both directions.

```bash
./proxy-speedtest -direct --upload --upload-size cdn \
  -uH "x-upload-key: …" --output table

./proxy-speedtest -direct --upload --upload-size cdn \
  --upload-url "https://gateway.example.com/my-bucket/speedtest/probe.bin" \
  -uH "x-upload-key: …"
```

The test rewrites **the same object** over and over for the whole duration — one PUT per 4 MiB
(`uploadChunkDefault`, overridable per endpoint via `UploadProfile.ChunkBytes`). Total bytes
written = duration × achieved throughput, so a 15 s run at 500 MB/s is ~7.5 GB and ~1 900 PUT
operations. Point it at a throwaway key and add a lifecycle rule if the bucket is versioned.

The 4 MiB chunk is the one real trade-off: larger chunks mean fewer (billed) write operations, but
a chunk that cannot finish inside the test duration never completes, and only 2xx responses count
toward throughput — so on a 1 MiB/s link a 32 MiB chunk would report 0 B/s. Smaller chunks make
per-request overhead eat into the measurement.

### Output formats

`-output` (alias `-o`) takes a **comma-separated list** of formats: `json`, `csv`, `text`, `table`,
`pic`, `none`.

| Format | Content | Default destination |
|--------|---------|---------------------|
| `json` | full result (nodes + options + traffic + duration + counts) | stdout |
| `csv`  | one row per node (machine-friendly; speeds in bytes/sec) | stdout |
| `text` | share links of working nodes only (importable as a subscription) | stdout |
| `table`| human-readable aligned table + summary footer | stdout |
| `pic`  | PNG result image | file (never stdout) |
| `none` | run the test, produce no output | — |

**Where output goes:**

- Data formats (`json`/`csv`/`text`/`table`):
  - no `-output-file` and a single format → **stdout**;
  - no `-output-file` and multiple formats → each written to `speedtest-<timestamp>.<ext>`;
  - `-output-file PATH` → the first data format uses `PATH`, extras auto-named.
- `pic` always writes a file: `-output-pic PATH`, else `-output-file` when it is the only output,
  else `speedtest-<timestamp>.png`.

**CSV columns:**
`id, group, remarks, protocol, ping_ms, avg_download_bytes_per_sec, max_download_bytes_per_sec,
avg_upload_bytes_per_sec, max_upload_bytes_per_sec, traffic_bytes, success, link`

### Command line options

| Option | Default | Description |
|--------|---------|-------------|
| `-test` | | Subscription URL or file path to test |
| `-direct` | false | Test without any proxy or subscription: measure this machine's own download/upload speed |
| `-timeout` | 16 | Timeout for each node test (seconds) |
| `-concurrency` | 2 | Number of concurrent tests |
| `-output`, `-o` | json | Output formats (comma-separated): json, csv, text, table, pic, none |
| `-output-file`, `-f` | | File path for the (first) data output |
| `-output-pic` | | File path for the PNG result image |
| `-mode` | all | Test mode: pingonly, speedonly, all |
| `-threads` | 1 | Parallel download connections per node |
| `-download-size` | | Download endpoint preset: cloudflare, hetzner-de, hetzner-us, linode-jp, vultr-sg, ovh-eu, datapacket-us, huawei-cn, worker |
| `-download-url` | | Custom download URL for speed test |
| `-upload`, `-upload-enable` | false | Also test upload speed after download |
| `-upload-size` | | Upload endpoint preset: cloudflare, dlptest, worker, cdn |
| `-upload-url` | | Custom upload URL (HTTP POST sink) |
| `-H` | | HTTP header for download and upload; repeatable (`Name: Value`) |
| `-dH` | | Download-only HTTP header; repeatable (`Name: Value`) |
| `-uH` | | Upload-only HTTP header; repeatable (`Name: Value`) |
| `-config` | | Config file path (JSON format) |
| `-log-level` | info | Log level: debug, info, warning, error, silent (silent also hides progress) |
| `-p` | 10888 | Web server port (CLI mode ignores it) |
| `-grpc` | false | Start as gRPC server |
| `-v` | | Show version |

### Config options

```json
{
    "group": "job",              // group name
    "speedtestMode": "pingonly", // speedonly pingonly all
    "pingMethod": "googleping",  // googleping tcpping
    "sortMethod": "rspeed",      // speed rspeed ping rping
    "concurrency": 1,            // concurrency number
    "testMode": 2,               // 2: ALLTEST 3: RETEST
    "subscription": "subscription url",
    "directMode": false,         // true: no proxy at all, test this machine's own bandwidth
    "timeout": 16,               // timeout in seconds
    "language": "en",            // en cn
    "fontSize": 24,
    "unique": true,              // remove duplicated value
    "theme": "rainbow",
    "outputMode": 1,             // 0: base64 1: pic path 2: no pic 3: json 4: txt
    "uploadEnable": true,
    "downloadUrl": "https://speed.example.com/__down?bytes=50000000",
    "uploadUrl": "https://speed.example.com/__up",
    "downloadHeaders": [{"name": "X-Speedtest-Key", "value": "secret"}],
    "uploadHeaders": [{"name": "X-Speedtest-Key", "value": "secret"}]
}
```

CLI headers are merged on top of config-file headers. `-H` applies to both directions, while `-dH`
and `-uH` override a same-named common header for download and upload respectively. Header names are
matched case-insensitively.

### Run as a gRPC server

```bash
# start the grpc server
./proxy-speedtest -grpc -p 10999

# grpc go client example in ./api/rpc/liteclient/client.go
# grpc python client example in ./api/rpc/liteclientpy/client.py
```

## Build

```bash
# require go >= 1.25, nodejs >= 24

# build frontend
npm install --prefix web/gui
npm run --prefix web/gui build

# build binary
go build
```

## Docker

```bash
docker build --network=host -t proxy-speedtest:$(git describe --tags --abbrev=0) .
docker run -p 10888:10888/tcp proxy-speedtest:$(git describe --tags --abbrev=0)
```

## Deploy under a sub-path

The frontend is built with a relative base (`base: './'` in `web/gui/vite.config.ts`), so one `dist`
works both at a domain root and under a prefix — assets, the JSON endpoints and the `/test` WebSocket
are all resolved against the page's own URL (`web/gui/src/lib/api.ts`), with no rebuild per mount point.

The server only registers root-level routes, so the reverse proxy has to **strip the prefix**, and the
page must be entered **with a trailing slash**:

```nginx
location = /speedtest { return 301 /speedtest/; }    # relative URLs need the trailing slash

location /speedtest/ {
    proxy_pass http://127.0.0.1:10888/;              # trailing slash here = strip the prefix
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;          # /test is a WebSocket
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_read_timeout 3600s;                        # a run holds the socket open for minutes
    proxy_buffering off;
}
```

Caddy equivalent: `handle_path /speedtest/* { reverse_proxy 127.0.0.1:10888 }`.

Start the backend with an explicit `-p 10888`: the 10888 default only applies when the binary is
invoked with no arguments at all, otherwise `-p` falls back to 8090.

## Credits

- [xxf098](https://github.com/xxf098/LiteSpeedTest)
- [clash](https://github.com/Dreamacro/clash)
- [xray-core](https://github.com/XTLS/Xray-core)
- [stairspeedtest-reborn](https://github.com/tindy2013/stairspeedtest-reborn)
- [gg](https://github.com/fogleman/gg)

## Developer

```golang
import (
    "context"
    "fmt"
    "time"

    "github.com/1orz/proxy-speedtest/web"
)

// see more details in ./examples
func testPing() error {
    ctx := context.Background()
    link := "https://www.example.com/subscription/link"
    opts := web.ProfileTestOptions{
        GroupName:     "Default",
        SpeedTestMode: "pingonly",   // pingonly speedonly all
        PingMethod:    "googleping", // googleping tcpping
        SortMethod:    "rspeed",     // speed rspeed ping rping
        Concurrency:   2,
        TestMode:      2,
        Subscription:  link,
        Language:      "en", // en cn
        FontSize:      24,
        Theme:         "rainbow",
        Unique:        true,
        Timeout:       10 * time.Second,
        OutputMode:    0,
    }
    nodes, err := web.TestContext(ctx, opts, &web.EmptyMessageWriter{})
    if err != nil {
        return err
    }
    // get all ok profile
    for _, node := range nodes {
        if node.IsOk {
            fmt.Println(node.Remarks)
        }
    }
    return nil
}
```
