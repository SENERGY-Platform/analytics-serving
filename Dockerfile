FROM golang:1.25 AS builder

ARG VERSION=0.0.35

COPY . /go/src/app
WORKDIR /go/src/app

ENV GO111MODULE=on

RUN CGO_ENABLED=0 GOOS=linux go build -o app -ldflags="-X 'main.version=$VERSION'" main.go

FROM alpine:latest

LABEL org.opencontainers.image.source=https://github.com/SENERGY-Platform/analytics-serving

WORKDIR /root/
COPY --from=builder /go/src/app/app .
COPY --from=builder /go/src/app/docs docs

HEALTHCHECK --interval=10s --timeout=5s --retries=3 CMD wget -nv -t1 --spider 'http://localhost/health-check' || exit 1

ENTRYPOINT ["./app"]