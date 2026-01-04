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
./lite

# start with another port
./lite -p 10889

# test in command line only mode
./lite --test https://example.com/subscription

# test in command line only mode with custom config
./lite --config config.json --test https://example.com/subscription
# details can find here https://github.com/1orz/proxy-speedtest/blob/master/config.json
```

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
./lite -grpc -p 10999

# grpc go client example in ./api/rpc/liteclient/client.go
# grpc python client example in ./api/rpc/liteclientpy/client.py
```

### Run as a HTTP/SOCKS5 proxy

```bash
# use default port 8090
./lite vmess://...
./lite vless://...
./lite trojan://...
./lite ss://...

# use another port
./lite -p 8091 vmess://...
```

## Build

```bash
# require go >= 1.25, nodejs >= 24

# build frontend
cp $(go env GOROOT)/misc/wasm/wasm_exec.js ./web/gui/wasm_exec.js
npm install --prefix web/gui
npm run --prefix web/gui build
GOOS=js GOARCH=wasm go build -o ./web/gui/dist/main.wasm ./wasm

# build binary
go build -o lite
```

## Docker

```bash
docker build --network=host -t lite:$(git describe --tags --abbrev=0) -f ./docker/Dockerfile ./
docker run -p 10888:10888/tcp lite:$(git describe --tags --abbrev=0)
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
