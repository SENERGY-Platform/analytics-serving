FROM golang

RUN go get -u github.com/golang/dep/cmd/dep

COPY . /go/src/analytics-serving
WORKDIR /go/src/analytics-serving

RUN dep ensure
RUN go build

EXPOSE 5001

CMD ./analytics-serving
