#!/bin/sh

DOWNLOAD_URL=$(curl -s https://api.github.com/repos/1orz/proxy-speedtest/releases/latest \
        | grep browser_download_url \
        | grep linux-amd64 \
        | grep -v v3- \
        | cut -d '"' -f 4)

test -z "$TMPDIR" && TMPDIR="$(mktemp -d)"

(
	cd "$TMPDIR"
	echo "Downloading lite from $DOWNLOAD_URL"
	curl -sL -o lite.gz "$DOWNLOAD_URL"
)

export TAR_FILE="${TMPDIR}/$(ls $TMPDIR | grep gz)"
gzip -d "$TAR_FILE"
mv "${TMPDIR}/lite" ./
chmod +x lite