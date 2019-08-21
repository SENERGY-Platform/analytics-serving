FROM golang:1.12

COPY . /go/src/analytics-serving
WORKDIR /go/src/analytics-serving

ENV GO111MODULE=on

RUN go build

EXPOSE 8000

CMD ./analytics-serving
