DESTDIR ?= $(shell go env GOPATH)/bin
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= dev

arches=amd64 arm arm64
shims=$(foreach arch,$(arches),pkg/runtimes/bin/exe.$(arch))

all: dist

pkg/runtimes/bin/exe.%: pkg/runtimes/shim/*.go
	env GOOS=linux GOARCH=$* CGO_ENABLED=0 go build -ldflags "-s -w" -o $@ ./pkg/runtimes/shim

hack/vendor: go.mod go.sum
	go mod vendor -o $@

pkg/proto/%.pb.go: hack/vendor proto/%.proto
	protoc -I=./proto -I=hack/vendor --go_out=. --go-grpc_out=. proto/$*.proto

nix/vendorSha256.txt: go.mod go.sum
	./hack/get-nix-vendorsha > $@

.PHONY: shims
shims: $(shims)
	which upx # required for compressing shims
	upx $(shims) || true # swallow AlreadyPackedException :/

.PHONY: dist
dist:
	mkdir -p ./dist/
	env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$(VERSION)" -o ./dist/bass ./cmd/bass

.PHONY: install
install: shims dist
	mkdir -p $(DESTDIR)
	cp ./dist/bass $(DESTDIR)

.PHONY: proto
proto: pkg/proto/bass.pb.go pkg/proto/runtime.pb.go pkg/proto/memo.pb.go

.PHONY: clean
clean:
	rm -f ./dist/ $(shims)
