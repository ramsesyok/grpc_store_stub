PROTO_DIR := proto
GEN_DIR   := gen

OS      ?= linux
ARCH    ?= amd64
VERSION ?= 0.0.0

EXT     := $(if $(filter windows,$(OS)),.exe,)
DIST_DIR := dist/$(VERSION)-$(OS)-$(ARCH)
ZIP_NAME := dist/grpc-store-stub-$(VERSION)-$(OS)-$(ARCH).zip

.PHONY: proto tidy build run-server run-client dist clean

proto:
	mkdir -p $(GEN_DIR)/simulation
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/simulation/simulation.proto

tidy:
	go mod tidy

build:
	go build -o bin/stub-server ./server
	go build -o bin/support-client ./client

dist:
	mkdir -p $(DIST_DIR)
	GOOS=$(OS) GOARCH=$(ARCH) go build -o $(DIST_DIR)/stub-server$(EXT) ./server
	GOOS=$(OS) GOARCH=$(ARCH) go build -o $(DIST_DIR)/support-client$(EXT) ./client
	cd dist && zip -j $(notdir $(ZIP_NAME)) $(VERSION)-$(OS)-$(ARCH)/*
	rm -rf $(DIST_DIR)
	@echo "Created: $(ZIP_NAME)"

run-server:
	go run ./server

run-client:
	go run ./client

clean:
	rm -rf bin/ dist/
