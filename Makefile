export GO111MODULE=on
BINARY_NAME=analytics-serving

all: deps build
install:
	go install cmd/analytics-serving/analytics-serving.go
build:
	CGO_ENABLED=0 go build cmd/analytics-serving/analytics-serving.go
test:
	go test -v ./...
clean:
	go clean
	rm -f $(BINARY_NAME)
deps:
	go build -v ./...
upgrade:
	go get -u