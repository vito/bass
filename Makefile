DESTDIR ?= /usr/local/bin

all: out/bass

pkg/runtimes/shim/exe: pkg/runtimes/shim/main.go
	env CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags "-s -w" -o $@ ./pkg/runtimes/shim/

out/bass: pkg/runtimes/shim/exe
	go build -o $@ ./cmd/bass

install: out/bass
	cp out/bass $(DESTDIR)/

clean:
	rm -f out/bass pkg/runtimes/shim/exe
