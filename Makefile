.PHONY : build run fresh test clean

BIN := ebs-exporter
HASH := $(shell git rev-parse --short HEAD)
COMMIT_DATE := $(shell git show -s --format=%ci ${HASH})
BUILD_DATE := $(shell date '+%Y-%m-%d %H:%M:%S')
VERSION := ${HASH} (${COMMIT_DATE})

build:
	go build -o ${BIN} -ldflags="-s -w -X 'main.buildVersion=${VERSION}' -X 'main.buildDate=${BUILD_DATE}'"

clean:
	go clean
	- rm -f ${BIN}

dist:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(MAKE) build

fresh: clean build run

lint:
	gofmt -s -w .
	find . -name "*.go" -exec golint {} \;

run:
	./${BIN}

test: lint
	go test
