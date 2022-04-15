DESTDIR ?= $(shell go env GOPATH)/bin
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= dev

arches=amd64 arm arm64
shims=$(foreach arch,$(arches),pkg/runtimes/bin/exe.$(arch))

all: cmd/bass/bass pkg/pb/bass.pb.go

pkg/runtimes/bin/exe.%: pkg/runtimes/shim/main.go
	env GOOS=linux GOARCH=$* CGO_ENABLED=0 go build -ldflags "-s -w" -o $@ ./pkg/runtimes/shim

cmd/bass/bass: shims
	env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$(VERSION)" -o ./cmd/bass/bass ./cmd/bass

pkg/proto/%.pb.go: proto/%.proto
	protoc -I=./proto --go_out=. --go-grpc_out=. proto/$*.proto

nix/vendorSha256.txt: go.mod go.sum
	./hack/get-nix-vendorsha > $@

.PHONY: shims
shims: $(shims)
	upx $(shims) || true # swallow AlreadyPackedException :/

.PHONY: install
install: cmd/bass/bass
	mkdir -p $(DESTDIR)
	cp $< $(DESTDIR)

.PHONY: proto
proto: pkg/proto/bass.pb.go pkg/proto/runtime.pb.go pkg/proto/progress.pb.go

.PHONY: clean
clean:
	rm -f cmd/bass/bass $(shims)
