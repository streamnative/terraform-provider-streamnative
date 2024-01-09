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

build: fmtcheck
	go build -o ${BINARY}

build-dev: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${PKG_NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${PKG_NAME}/${VERSION}/${OS_ARCH}

testacc: fmtcheck
	TF_ACC=1 go test $(TEST) -v -count 3 $(TESTARGS) -timeout 120m

fmt:
	@echo "==> Fixing source code with gofmt..."
	@gofmt -s -w cloud

# Currently required by tf-deploy compile
fmtcheck:
	@sh -c "'$(CURDIR)/scripts/gofmtcheck.sh'"

lint:
	@echo "==> Checking source code against linters..."
	@golangci-lint run -c .golangci.yaml ./...
	@tfproviderlint \
		-c 1 \
		-AT001 \
		-AT002 \
		-S001 \
		-S002 \
		-S003 \
		-S004 \
		-S005 \
		-S007 \
		-S008 \
		-S009 \
		-S010 \
		-S011 \
		-S012 \
		-S013 \
		-S014 \
		-S015 \
		-S016 \
		-S017 \
		-S019 \
		./cloud

tools:
	go install github.com/bflad/tfproviderlint/cmd/tfproviderlint@v0.29.0
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.46.2

.PHONY: build