FROM golang:1.13

COPY . /go/src/analytics-serving
WORKDIR /go/src/analytics-serving

ENV GO111MODULE=on

RUN make build

EXPOSE 8000

CMD ./analytics-serving
