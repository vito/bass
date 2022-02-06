DESTDIR ?= /usr/local/bin
GOOS ?= linux
GOARCH ?= amd64

arches=amd64 arm arm64 ppc64le riscv64 s390x
shims=$(foreach arch,$(arches),pkg/runtimes/shim/bin/exe.$(arch))
targets=cmd/bass/bass $(shims)

pkg/runtimes/shim/bin/exe.%: pkg/runtimes/shim/main.go
	env GOOS=linux GOARCH=$* CGO_ENABLED=0 go build -ldflags "-s -w" -o $@ ./pkg/runtimes/shim/

.PHONY: cmd/bass/bass
cmd/bass/bass: $(shims)
	env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ./cmd/bass/bass ./cmd/bass

.PHONY: install
install: $(shims)
	env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go install -trimpath ./cmd/bass

.PHONY: clean
clean:
	rm -f $(targets)
