FROM --platform=${BUILDPLATFORM} golang:1.26-alpine AS builder

RUN apk add git

WORKDIR /go/src/app
COPY . .

ARG TARGETOS TARGETARCH TARGETVARIANT

ENV CGO_ENABLED=0
RUN go get \
    && go mod download \
    && GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#"v"} go build -a -o rtsp-to-web

FROM alpine:3.23

WORKDIR /app

RUN apk add --no-cache tzdata
ENV TZ=Asia/Shanghai

COPY --from=builder /go/src/app/rtsp-to-web /app/
COPY --from=builder /go/src/app/web /app/web

RUN mkdir -p /config
COPY --from=builder /go/src/app/config.example.json /config/config.example.json

ENV GO111MODULE="on"
ENV GIN_MODE="release"

CMD ["sh", "-c", "if [ -f /config/config.json ]; then exec ./rtsp-to-web --config=/config/config.json; else exec ./rtsp-to-web --config=/config/config.example.json; fi"]
