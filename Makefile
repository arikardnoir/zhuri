BINARY  := zhuri
CMD     := ./cmd/zhuri

VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Standard FHS install location. Override PREFIX for a user-local install
# (e.g. `make install PREFIX=$$HOME/.local`) and DESTDIR for staged/packaged
# builds (e.g. `make install DESTDIR=/tmp/pkgroot`).
PREFIX  ?= /usr/local
DESTDIR ?=
BINDIR  := $(DESTDIR)$(PREFIX)/bin

GO ?= go

GO_SOURCES := $(shell find cmd detect fix report walk -name '*.go') go.mod go.sum

.PHONY: all build install uninstall test bench vet fmt-check check clean

all: build

build: $(BINARY)

# Depending on the actual binary file (not a .PHONY target) means `sudo make
# install` right after a plain `make build` reuses the binary you already
# built as yourself instead of trying to recompile as root - which usually
# fails, since `sudo` resets PATH and root can't see your `go` binary.
#
# CGO_ENABLED=0 gives a static binary with no libc dependency, so it runs
# unmodified on any Linux distro regardless of glibc/musl version.
$(BINARY): $(GO_SOURCES)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

install: $(BINARY)
	install -d $(BINDIR)
	install -m 0755 $(BINARY) $(BINDIR)/$(BINARY)
	@echo "instalado em $(BINDIR)/$(BINARY)"

uninstall:
	rm -f $(BINDIR)/$(BINARY)

test:
	$(GO) test ./...

bench:
	$(GO) test ./... -run '^$$' -bench=. -benchmem

vet:
	$(GO) vet ./...

fmt-check:
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then \
		echo "gofmt precisa de ser corrido em:"; echo "$$out"; exit 1; \
	fi

check: fmt-check vet test

clean:
	rm -f $(BINARY)
