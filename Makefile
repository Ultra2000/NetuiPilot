VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build install uninstall clean

build:
	go build -ldflags "$(LDFLAGS)" -o netuipilot ./cmd/netuipilot/

install: build
	sudo install -m 755 netuipilot /usr/local/bin/netuipilot

uninstall:
	sudo rm -f /usr/local/bin/netuipilot

clean:
	rm -f netuipilot
