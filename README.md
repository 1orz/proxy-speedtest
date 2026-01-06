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

### Run as a speed test tool

```bash
# run this command then open http://127.0.0.1:10888/ in your browser to start speed test
./proxy-speedtest

# start with another port
./proxy-speedtest -p 10889

# test in command line only mode (output JSON by default)
./proxy-speedtest --test https://example.com/subscription

# test with custom timeout (seconds) and concurrency
./proxy-speedtest --test https://example.com/subscription --timeout 20 --concurrency 4

# test with custom download file size (10mb, 100mb, cloudflare10, cloudflare100)
./proxy-speedtest --test https://example.com/subscription --download-size 100mb

# test with custom download URL
./proxy-speedtest --test https://example.com/subscription --download-url "https://speed.cloudflare.com/__down?bytes=50000000"

# output formats: json (default), text, pic, none
./proxy-speedtest --test https://example.com/subscription --output json
./proxy-speedtest --test https://example.com/subscription --output text

# test with config file
./proxy-speedtest --config config.json --test https://example.com/subscription
```

### Command line options

| Option | Default | Description |
|--------|---------|-------------|
| `-test` | | Subscription URL or file path to test |
| `-timeout` | 16 | Timeout for each node test (seconds) |
| `-concurrency` | 2 | Number of concurrent tests |
| `-output` | json | Output format: json, text, pic, none |
| `-download-size` | | Download size: 10mb, 100mb, cloudflare10, cloudflare100 |
| `-download-url` | | Custom download URL for speed test |
| `-config` | | Config file path (JSON format) |
| `-p` | 8090 | Port for web server or proxy |
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

### Run as a HTTP/SOCKS5 proxy

```bash
# use default port 8090
./proxy-speedtest vmess://...
./proxy-speedtest vless://...
./proxy-speedtest trojan://...
./proxy-speedtest ss://...

# use another port
./proxy-speedtest -p 8091 vmess://...
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
