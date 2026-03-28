// Package main は SimulationService の gRPC サーバーです。
// TCP ポート 50051 でリッスンし、シミュレーションリクエストを処理します。
package main

import (
	"log"
	"net"

	simpb "github.com/ramsesyok/grpc-store-stub/gen/simulation"
	"google.golang.org/grpc"
)

// main は gRPC サーバーを起動します。
// TCP ポート 50051 でリッスンし、SimulationService を登録します。
func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	simpb.RegisterSimulationServiceServer(s, &simServer{})
	log.Println("server listening on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
