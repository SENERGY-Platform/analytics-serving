FROM golang:1.23 AS builder

COPY . /go/src/app
WORKDIR /go/src/app

ENV GO111MODULE=on

RUN make build

RUN git log -1 --oneline > version.txt

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /go/src/app/analytics-serving .
COPY --from=builder /go/src/app/version.txt .

EXPOSE 8000

LABEL org.opencontainers.image.source=https://github.com/SENERGY-Platform/analytics-serving

ENTRYPOINT ["./analytics-serving"]
