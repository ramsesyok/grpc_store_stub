package main

import (
	"context"
	"io"
	"log"
	"time"

	simpb "github.com/ramsesyok/grpc-store-stub/gen/simulation"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"

	"os"
)

// runFlags は run サブコマンドのフラグ値を保持します。
var runFlags struct {
	// input: 読み込むリクエスト JSON ファイルのパス
	input string
	// server: 接続先の gRPC サーバーアドレス
	server string
}

// runCmd は「run」サブコマンドの定義です。
// JSON ファイルからリクエストを読み込み、gRPC サーバーへ送信します。
// server streaming でレスポンスを受信し、各チャンクの情報をログに出力します。
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "JSON ファイルからリクエストを読み込んでシミュレーションを実行する",
	Long: `指定した JSON ファイルを SimulationRequest としてサーバーへ送信し、
server streaming で返されるレスポンスを受信してログに出力します。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSimulation(runFlags.input, runFlags.server)
	},
}

func init() {
	// --input フラグ: 読み込むリクエスト JSON ファイルのパス
	runCmd.Flags().StringVarP(&runFlags.input, "input", "i", "request.json", "リクエスト JSON ファイルのパス")
	// --server フラグ: gRPC サーバーのアドレス
	runCmd.Flags().StringVarP(&runFlags.server, "server", "s", "localhost:50051", "gRPC サーバーのアドレス")

	rootCmd.AddCommand(runCmd)
}

// runSimulation は JSON ファイルを読み込んで gRPC サーバーへシミュレーションをリクエストし、
// ストリームで返されるチャンクを順に受信してログへ出力します。
func runSimulation(input, server string) error {
	// JSON ファイルを読み込む
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}

	// protojson.Unmarshal で JSON を SimulationRequest へデシリアライズする
	// camelCase・snake_case どちらの JSON キー形式にも対応している
	req := &simpb.SimulationRequest{}
	if err := protojson.Unmarshal(data, req); err != nil {
		return err
	}

	// insecure.NewCredentials() で TLS なしの接続を確立する（開発・検証用）
	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	c := simpb.NewSimulationServiceClient(conn)

	// シミュレーション時間が長くなる可能性があるため、タイムアウトを 5 分に設定する
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// server streaming RPC を開始する
	stream, err := c.RunSimulation(ctx, req)
	if err != nil {
		return err
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
			return err
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
	return nil
}
