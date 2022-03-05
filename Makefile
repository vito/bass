DESTDIR ?= $(shell go env GOPATH)/bin
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

arches=amd64 arm arm64
shims=$(foreach arch,$(arches),pkg/runtimes/bin/exe.$(arch))
targets=cmd/bass/bass $(shims)

all: cmd/bass/bass

pkg/runtimes/bin/exe.%: pkg/runtimes/shim/go.mod pkg/runtimes/shim/main.go
	cd pkg/runtimes/shim/ && env GOOS=linux GOARCH=$* CGO_ENABLED=0 go build -ldflags "-s -w" -o ../bin/$(notdir $@) .

.PHONY: cmd/bass/bass
cmd/bass/bass: $(shims)
	upx $(shims) || true # swallow AlreadyPackedException :/
	env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -trimpath -o ./cmd/bass/bass ./cmd/bass

.PHONY: install
install: cmd/bass/bass
	cp $< $(DESTDIR)

.PHONY: clean
clean:
	rm -f $(targets)
