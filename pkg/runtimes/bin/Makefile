.PHONY: all clean

ARCHS := amd64 arm arm64
EXES := $(addprefix exe.,$(ARCHS))

all: $(EXES)

$(EXES): exe.%:
	@echo "Building for $*"
	@GOOS=linux GOARCH=$* CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $@ ../shim
	@echo "Compressing $@"
	@upx $@ || true # swallow AlreadyPackedException

clean:
	@rm -f $(EXES)
