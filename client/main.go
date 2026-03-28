// Package main は SimulationService の gRPC クライアントです。
// 2 つのサブコマンドを提供します。
//
//	run  JSONファイルを読み込んでサーバーへリクエストを送信し、ストリームを受信する
//	gen  シミュレーション用リクエスト JSON ファイルを生成する
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	simpb "github.com/ramsesyok/grpc-store-stub/gen/simulation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

// main はサブコマンドを解析してそれぞれの処理へ振り分けます。
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  client run --input <file> [--server <addr>]")
		fmt.Fprintln(os.Stderr, "  client gen --output <file> [--objects <n>] [--width <w>] [--height <h>]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "gen":
		genCmd(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// runCmd は「run」サブコマンドの処理です。
// JSON ファイルからリクエストを読み込み、gRPC サーバーへ送信します。
// server streaming でレスポンスを受信し、各チャンクの情報をログに出力します。
func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	// --input: 読み込むリクエスト JSON ファイルのパス
	input := fs.String("input", "request.json", "path to request JSON file")
	// --server: 接続先の gRPC サーバーアドレス
	server := fs.String("server", "localhost:50051", "server address")
	fs.Parse(args)

	// JSON ファイルを読み込む
	data, err := os.ReadFile(*input)
	if err != nil {
		log.Fatalf("failed to read %s: %v", *input, err)
	}

	// protojson.Unmarshal で JSON を SimulationRequest へデシリアライズする
	// camelCase・snake_case どちらの JSON キー形式にも対応している
	req := &simpb.SimulationRequest{}
	if err := protojson.Unmarshal(data, req); err != nil {
		log.Fatalf("failed to parse request JSON: %v", err)
	}

	// insecure.NewCredentials() で TLS なしの接続を確立する（開発・検証用）
	conn, err := grpc.NewClient(*server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	c := simpb.NewSimulationServiceClient(conn)

	// シミュレーション時間が長くなる可能性があるため、タイムアウトを 5 分に設定する
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// server streaming RPC を開始する
	stream, err := c.RunSimulation(ctx, req)
	if err != nil {
		log.Fatalf("RunSimulation failed: %v", err)
	}

	// ストリームからチャンクを順番に受信する
	chunkNum := 0
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			// サーバーがストリームを正常に閉じた
			break
		}
		if err != nil {
			log.Fatalf("stream error: %v", err)
		}
		chunkNum++
		log.Printf("[chunk %d] itemCount=%d range=[%.2f, %.2f] isFinal=%v",
			chunkNum,
			resp.GetItemCount(),
			resp.GetRange().GetStart(),
			resp.GetRange().GetEnd(),
			resp.GetIsFinal(),
		)
		// isFinal = true のチャンクを受信したらストリームの終了と見なして抜ける
		if resp.GetIsFinal() {
			break
		}
	}
	log.Printf("done. total chunks: %d", chunkNum)
}

// genCmd は「gen」サブコマンドの処理です。
// 指定されたパラメータに基づいてシミュレーション用リクエスト JSON を生成し、
// ファイルへ保存します。オブジェクトの初期位置はエリア内でランダムに配置されます。
func genCmd(args []string) {
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	// --output: 出力先のファイルパス
	output := fs.String("output", "request.json", "output file path")
	// --objects: 生成するオブジェクト数
	objects := fs.Int("objects", 1, "number of objects")
	// --width / --height: エリアのサイズ（x: 0〜width、y: 0〜height）
	width  := fs.Float64("width",  100.0, "area width  (x: 0 to width)")
	height := fs.Float64("height", 100.0, "area height (y: 0 to height)")
	// --range-start / --range-end: シミュレーション時間の開始・終了（秒）
	rangeStart := fs.Float64("range-start", 0.0,    "simulation range start (seconds)")
	rangeEnd   := fs.Float64("range-end",   1000.0, "simulation range end   (seconds)")
	// --interval: シミュレーションの計算間隔（秒）。実時間ではなくシミュレーション上の時間
	interval := fs.Float64("interval", 1.0, "simulation step interval (seconds)")
	// --bulk-size: 1 チャンクあたりの送信アイテム数
	bulkSize := fs.Int("bulk-size", 100, "number of items per response chunk")
	// --wait: チャンク送信間のスリープ時間（秒）。送信レートが高すぎる場合に調整する
	wait := fs.Float64("wait", 0.5, "sleep between sends (seconds)")
	fs.Parse(args)

	// 乱数源を現在時刻で初期化する（実行ごとに異なる配置を生成する）
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 各オブジェクトの初期状態をランダムに生成する
	objs := make([]*simpb.SimObject, *objects)
	for i := range objs {
		objs[i] = &simpb.SimObject{
			Id: int32(i),
			// x, y はエリア内のランダムな位置
			X: rng.Float64() * *width,
			Y: rng.Float64() * *height,
			// z はデータ量調整用。height をスケールとしてランダムな値を設定する
			Z:         rng.Float64() * *height,
			// direction は 0〜360° のランダムな角度
			Direction: rng.Float64() * 360.0,
			// speed は固定値（5.0）とする
			Speed: 5.0,
		}
	}

	// リクエストメッセージを組み立てる
	req := &simpb.SimulationRequest{
		Objects: objs,
		Area: &simpb.Area{
			// エリアの min は 0 固定、max は width/height で指定する
			XMin: 0, XMax: *width,
			YMin: 0, YMax: *height,
		},
		Range:    &simpb.Range{Start: *rangeStart, End: *rangeEnd},
		Interval: *interval,
		BulkSize: int32(*bulkSize),
		Wait:     *wait,
	}

	// protojson でインデント付き JSON へシリアライズする
	// フィールド名は proto の命名規則に従い camelCase で出力される（例: bulkSize）
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(req)
	if err != nil {
		log.Fatalf("failed to marshal request: %v", err)
	}

	// JSON をファイルへ書き出す
	if err := os.WriteFile(*output, data, 0644); err != nil {
		log.Fatalf("failed to write %s: %v", *output, err)
	}
	log.Printf("generated %s (%d objects, area %.0fx%.0f)", *output, *objects, *width, *height)
}
