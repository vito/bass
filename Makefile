DESTDIR ?= $(shell go env GOPATH)/bin
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= dev

arches=amd64 arm arm64
shims=$(foreach arch,$(arches),pkg/runtimes/bin/exe.$(arch))
targets=cmd/bass/bass nix/vendorSha256.txt $(shims)

all: cmd/bass/bass

pkg/runtimes/bin/exe.%: pkg/runtimes/shim/main.go
	env GOOS=linux GOARCH=$* CGO_ENABLED=0 go build -ldflags "-s -w" -o $@ ./pkg/runtimes/shim

cmd/bass/bass: $(shims)
	upx $(shims) || true # swallow AlreadyPackedException :/
	env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$(VERSION)" -o ./cmd/bass/bass ./cmd/bass

nix/vendorSha256.txt: go.mod go.sum
	./hack/get-nix-vendorsha > $@

.PHONY: shims
shims: $(shims)

.PHONY: install
install: cmd/bass/bass
	mkdir -p $(DESTDIR)
	cp $< $(DESTDIR)

.PHONY: clean
clean:
	rm -f $(targets)
