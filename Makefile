DESTDIR ?= /usr/local/bin
GOOS ?= linux
GOARCH ?= amd64

targets=out/bass pkg/runtimes/shim/bin/exe.$(GOARCH)

all: out/bass

pkg/runtimes/shim/bin/exe.$(GOARCH): pkg/runtimes/shim/main.go
	env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -ldflags "-s -w" -o $@ ./pkg/runtimes/shim/

out/bass: pkg/runtimes/shim/bin/exe.$(GOARCH)
	go build -o $@ ./cmd/bass

install: out/bass
	cp out/bass $(DESTDIR)/

clean:
	rm -f $(targets)
