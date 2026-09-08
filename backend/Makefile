.PHONY: build test install

BIN_DIR ?= bin
PREFIX ?= /opt/likeadmin/backend

build:
	mkdir -p $(BIN_DIR)
	go build -trimpath -o $(BIN_DIR)/api ./cmd/api
	go build -trimpath -o $(BIN_DIR)/crontab ./cmd/crontab
	go build -trimpath -o $(BIN_DIR)/think ./cmd/think
	go build -trimpath -o $(BIN_DIR)/strangler ./cmd/strangler

test:
	go test ./...

install: build
	install -d $(PREFIX)/bin
	install -m 0755 $(BIN_DIR)/api $(PREFIX)/bin/api
	install -m 0755 $(BIN_DIR)/crontab $(PREFIX)/bin/crontab
	install -m 0755 $(BIN_DIR)/think $(PREFIX)/bin/think
	install -m 0755 $(BIN_DIR)/strangler $(PREFIX)/bin/strangler
