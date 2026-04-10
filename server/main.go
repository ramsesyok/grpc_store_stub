// Package main は SimulationService の gRPC サーバーです。
// TCP ポート 50051 でリッスンし、シミュレーションリクエストを処理します。
package main

import (
	"log"
	"net"
	"os"

	simpb "github.com/ramsesyok/grpc-store-stub/gen/simulation"
	"google.golang.org/grpc"
)

// localIPWithPort はアウトバウンド接続で使われるローカル IP アドレスを取得し、
// "IP:port" 形式の文字列を返します。取得できない場合は "unknown:port" を返します。
func localIPWithPort(port string) string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "unknown:" + port
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String() + ":" + port
}

// main は gRPC サーバーを起動します。
// TCP ポート 50051 でリッスンし、SimulationService を登録します。
func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	serverID := os.Getenv("SERVER_ID")
	if serverID == "" {
		serverID = localIPWithPort("50051")
	}
	s := grpc.NewServer()
	simpb.RegisterSimulationServiceServer(s, &simServer{
		jobs:     NewJobManager(),
		serverID: serverID,
	})
	log.Println("server listening on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
