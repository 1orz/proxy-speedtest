# sudo docker build --network=host -t lite:0.1 -f ./docker/Dockerfile ./
FROM golang:1.25 AS wasm
ADD / /wasm/lite
WORKDIR /wasm/lite
RUN cp $(go env GOROOT)/misc/wasm/wasm_exec.js ./web/gui/wasm_exec.js

FROM node:24 AS gui
ADD / /gui/lite
WORKDIR /gui/lite
COPY --from=wasm /wasm/lite/web/gui/wasm_exec.js /gui/lite/web/gui/wasm_exec.js 
RUN npm install --prefix web/gui
RUN npm run --prefix web/gui build

FROM golang:1.25 AS build
ADD / /go/lite
WORKDIR /go/lite
COPY --from=gui /gui/lite/web/gui/dist/ /go/lite/web/gui/dist
# china only
# RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod tidy
RUN GOOS=js GOARCH=wasm go build -o ./web/gui/dist/main.wasm ./wasm
RUN make dockerbin

FROM gcr.io/distroless/static-debian12
COPY --from=build /go/lite/bin/lite-dockerbin /
EXPOSE 10888
CMD [ "/lite-dockerbin" ]