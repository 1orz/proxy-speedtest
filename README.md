# LiteSpeedTest

LiteSpeedTest is a simple tool for batch testing proxy servers.

## Features

- Support VMess / VLESS / Trojan / Shadowsocks protocols
- Support subscription URLs and profile links
- Support Clash configuration files
- Support multiple transport protocols: TCP, WebSocket, gRPC, SplitHTTP/XHTTP, HTTP Upgrade
- Support TLS and Reality security

![build](https://github.com/1orz/proxy-speedtest/actions/workflows/ci.yaml/badge.svg?branch=master&event=push)

## Usage

### Web mode vs CLI mode

- **No subscription/link argument → web mode.** Starts the web UI (default `http://127.0.0.1:10888/`).
- **A subscription link, a share link (`vmess://` / `vless://` / `trojan://` / `ss://` …), or a local
  file → CLI mode.** Runs the speed test directly and prints/saves the result. Live progress goes to
  **stderr**; the result data goes to **stdout** (so it pipes cleanly).

```bash
# web mode: open http://127.0.0.1:10888/ in your browser
./proxy-speedtest
./proxy-speedtest -p 10889            # another port

# CLI mode: subscription (JSON to stdout by default)
./proxy-speedtest --test https://example.com/subscription

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

# use a config file
./proxy-speedtest --config config.json --test https://sub
```

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
| `-timeout` | 16 | Timeout for each node test (seconds) |
| `-concurrency` | 2 | Number of concurrent tests |
| `-output`, `-o` | json | Output formats (comma-separated): json, csv, text, table, pic, none |
| `-output-file`, `-f` | | File path for the (first) data output |
| `-output-pic` | | File path for the PNG result image |
| `-mode` | all | Test mode: pingonly, speedonly, all |
| `-threads` | 1 | Parallel download connections per node |
| `-download-size` | | Download endpoint preset: cloudflare, hetzner-de, hetzner-us, linode-jp, vultr-sg, ovh-eu, datapacket-us, huawei-cn, worker |
| `-download-url` | | Custom download URL for speed test |
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
    "timeout": 16,               // timeout in seconds
    "language": "en",            // en cn
    "fontSize": 24,
    "unique": true,              // remove duplicated value
    "theme": "rainbow",
    "outputMode": 1              // 0: base64 1: pic path 2: no pic 3: json 4: txt
}
```

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
