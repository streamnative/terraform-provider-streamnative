TEST?=./...
GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)
HOSTNAME=registry.terraform.io
NAMESPACE?=streamnative
PKG_NAME=streamnative
BINARY=terraform-provider-${PKG_NAME}
VERSION?=0.1.0
OS := $(if $(GOOS),$(GOOS),$(shell go env GOOS))
ARCH := $(if $(GOARCH),$(GOARCH),$(shell go env GOARCH))
OS_ARCH := ${OS}_${ARCH}

default: build

build:
	go build -o ${BINARY}

build-dev: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${PKG_NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${PKG_NAME}/${VERSION}/${OS_ARCH}

.PHONY: build