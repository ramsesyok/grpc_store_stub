PROTO_DIR := proto
GEN_DIR   := gen

.PHONY: proto tidy build run-server run-client clean

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
	go build -o bin/server ./server
	go build -o bin/client ./client

run-server:
	go run ./server

run-client:
	go run ./client

clean:
	rm -rf bin/
