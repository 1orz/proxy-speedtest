#!/usr/bin/env bash
#
# fresh-start.sh —— 从零本地全新构建并启动 proxy-speedtest(前端 + 后端一体)。
#
# 流程:检查依赖 → 构建 React 前端(web/gui/dist)→ 编译 Go 后端(内嵌前端)→ 启动 Web 服务。
#
# 用法:
#   ./fresh-start.sh                 # 全新构建并在 :10888 启动
#   ./fresh-start.sh -p 8080         # 指定端口
#   ./fresh-start.sh -b 0.0.0.0      # 绑定地址/网卡名(默认所有接口)
#   ./fresh-start.sh -c              # 先彻底清理(dist / node_modules / bin)再构建,最“从 0”
#   ./fresh-start.sh --no-run        # 只构建不启动
#   ./fresh-start.sh -h              # 帮助
#
set -euo pipefail

# 切到脚本所在目录(=仓库根),使脚本可从任意 cwd 调用。
cd "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

MODULE="github.com/1orz/proxy-speedtest"
BINDIR="bin"
BIN="${BINDIR}/proxy-speedtest"

PORT=10888
BIND=""
CLEAN=0
RUN=1

usage() { sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--port)  PORT="${2:?--port 需要参数}"; shift 2 ;;
    -b|--bind)  BIND="${2:?--bind 需要参数}"; shift 2 ;;
    -c|--clean) CLEAN=1; shift ;;
    --no-run)   RUN=0; shift ;;
    -h|--help)  usage ;;
    *) echo "未知参数: $1(用 -h 看帮助)" >&2; exit 2 ;;
  esac
done

log()  { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31mfatal:\033[0m %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# 1) 依赖检查
# ---------------------------------------------------------------------------
log "检查依赖…"
command -v go   >/dev/null 2>&1 || die "未找到 go,请安装 Go 1.26+"
command -v node >/dev/null 2>&1 || die "未找到 node,请安装 Node.js 20+"
command -v npm  >/dev/null 2>&1 || die "未找到 npm"
echo "    go   $(go version | awk '{print $3}')"
echo "    node $(node -v)"
echo "    npm  v$(npm -v)"

# ---------------------------------------------------------------------------
# 2) 可选:彻底清理(真正的从 0)
# ---------------------------------------------------------------------------
if [[ "$CLEAN" -eq 1 ]]; then
  log "清理旧产物(dist / node_modules / bin)…"
  rm -rf web/gui/dist web/gui/node_modules "$BINDIR"
fi

# ---------------------------------------------------------------------------
# 3) 构建前端 → web/gui/dist
# ---------------------------------------------------------------------------
log "安装并构建前端(web/gui)…"
pushd web/gui >/dev/null
npm install
npm run build
popd >/dev/null
[[ -f web/gui/dist/index.html ]] || die "前端构建产物缺失(web/gui/dist/index.html)"

# ---------------------------------------------------------------------------
# 4) 构建后端(把上一步的 dist 内嵌进二进制)
# ---------------------------------------------------------------------------
log "编译 Go 后端 → ${BIN}…"
VERSION="$(git describe --tags 2>/dev/null || echo dev)"
BUILD_TIME="$(date -u '+%Y-%m-%d_%H:%M:%S')"
LDFLAGS="-X '${MODULE}/constant.Version=${VERSION}' -X '${MODULE}/constant.BuildTime=${BUILD_TIME}' -w -s"
mkdir -p "$BINDIR"
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o "$BIN" .
log "构建完成:$("$BIN" -v 2>/dev/null || echo "$BIN")"

# ---------------------------------------------------------------------------
# 5) 启动
# ---------------------------------------------------------------------------
if [[ "$RUN" -eq 0 ]]; then
  log "已构建完成(--no-run,未启动)。手动启动:$BIN -p $PORT"
  exit 0
fi

ARGS=(-p "$PORT")
[[ -n "$BIND" ]] && ARGS+=(-bind "$BIND")

log "启动服务(Ctrl-C 停止)"
echo "    前端+后端: http://127.0.0.1:${PORT}"
exec "$BIN" "${ARGS[@]}"
