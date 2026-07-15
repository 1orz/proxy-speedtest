FROM node:26-alpine AS gui-builder

WORKDIR /build

COPY web/gui/package.json web/gui/package-lock.json* web/gui/pnpm-lock.yaml* web/gui/yarn.lock* ./web/gui/

RUN cd web/gui && \
    if [ -f pnpm-lock.yaml ]; then npm install -g pnpm && pnpm install --frozen-lockfile; \
    elif [ -f yarn.lock ]; then yarn install --frozen-lockfile; \
    elif [ -f package-lock.json ]; then npm ci; \
    else npm install; fi

COPY web/gui/ ./web/gui/
RUN cd web/gui && npm run build

FROM golang:1.26-alpine AS go-builder

ARG VERSION=unknown
ARG BUILD_TIME=unknown

RUN apk add --no-cache git make

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

COPY --from=gui-builder /build/web/gui/dist/ ./web/gui/dist/

RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-X 'github.com/1orz/proxy-speedtest/constant.Version=${VERSION}' \
              -X 'github.com/1orz/proxy-speedtest/constant.BuildTime=${BUILD_TIME}' \
              -w -s" \
    -o /build/proxy-speedtest .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=go-builder /build/proxy-speedtest /proxy-speedtest

EXPOSE 10888

USER nonroot:nonroot

ENTRYPOINT ["/proxy-speedtest"]

