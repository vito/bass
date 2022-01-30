DESTDIR ?= /usr/local/bin
GOOS ?= linux
GOARCH ?= amd64

arches=amd64 arm arm64 ppc64le riscv64 s390x
shims=$(foreach arch,$(arches),pkg/runtimes/shim/bin/exe.$(arch))
targets=out/bass $(shims)

.PHONY: all clean install

all: $(targets)

install: out/bass
	cp out/bass $(DESTDIR)/

clean:
	rm -f $(targets)

pkg/runtimes/shim/bin/exe.%: pkg/runtimes/shim/main.go
	env GOOS=$(GOOS) GOARCH=$* CGO_ENABLED=0 go build -ldflags "-s -w" -o $@ ./pkg/runtimes/shim/

out/bass: $(shims)
	env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o $@ ./cmd/bass
