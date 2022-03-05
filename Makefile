DESTDIR ?= /usr/local/bin
GOOS ?= linux
GOARCH ?= amd64

arches=amd64 arm arm64
shims=$(foreach arch,$(arches),pkg/runtimes/shim/bin/exe.$(arch))
targets=cmd/bass/bass $(shims)

all: cmd/bass/bass

pkg/runtimes/shim/bin/exe.%: pkg/runtimes/shim/go.mod pkg/runtimes/shim/main.go
	cd pkg/runtimes/shim/ && env GOOS=linux GOARCH=$* CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/$(notdir $@) .
	upx -q $@

pkg/runtimes/shim/go.mod: pkg/runtimes/shim/empty.go.mod
	cp $< $@

.PHONY: cmd/bass/bass
cmd/bass/bass: $(shims)
	rm -f pkg/runtimes/shim/go.mod # workaround go:embed
	env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ./cmd/bass/bass ./cmd/bass

.PHONY: install
install: $(shims)
	rm -f pkg/runtimes/shim/go.mod # workaround go:embed
	env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go install -trimpath ./cmd/bass

.PHONY: clean
clean:
	rm -f $(targets)
