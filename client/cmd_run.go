package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"time"

	simpb "github.com/ramsesyok/grpc-store-stub/gen/simulation"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

// runFlags は run サブコマンドのフラグ値を保持します。
var runFlags struct {
	// input: 読み込むリクエスト JSON ファイルのパス
	input string
	// server: 接続先の gRPC サーバーアドレス
	server string
	// debug: 受信レスポンスを NDJSON 形式で書き出す出力ファイルパス（空の場合は無効）
	debug string
}

// runCmd は「run」サブコマンドの定義です。
// JSON ファイルからリクエストを読み込み、gRPC サーバーへ送信します。
// server streaming でレスポンスを受信し、各チャンクの情報をログに出力します。
// --debug を指定した場合は、受信したレスポンスを NDJSON 形式でファイルへ保存します。
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "JSON ファイルからリクエストを読み込んでシミュレーションを実行する",
	Long: `指定した JSON ファイルを SimulationRequest としてサーバーへ送信し、
server streaming で返されるレスポンスを受信してログに出力します。
--debug を指定すると、受信した各チャンクを 1 行 1 JSON の NDJSON 形式で保存します。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSimulation(runFlags.input, runFlags.server, runFlags.debug)
	},
}

func init() {
	// --input フラグ: 読み込むリクエスト JSON ファイルのパス
	runCmd.Flags().StringVarP(&runFlags.input, "input", "i", "request.json", "リクエスト JSON ファイルのパス")
	// --server フラグ: gRPC サーバーのアドレス
	runCmd.Flags().StringVarP(&runFlags.server, "server", "s", "localhost:50051", "gRPC サーバーのアドレス")
	// --debug フラグ: 受信レスポンスを NDJSON で保存するファイルパス（省略時は保存しない）
	runCmd.Flags().StringVarP(&runFlags.debug, "debug", "d", "", "受信レスポンスを NDJSON 形式で保存するファイルパス")

	rootCmd.AddCommand(runCmd)
}

// runSimulation は JSON ファイルを読み込んで gRPC サーバーへシミュレーションをリクエストし、
// ストリームで返されるチャンクを順に受信してログへ出力します。
// debugPath が空でない場合は、各チャンクを 1 行 1 JSON の NDJSON 形式でファイルへ追記します。
func runSimulation(input, server, debugPath string) error {
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

	// --debug が指定されている場合は出力ファイルを開く
	// 既存ファイルは上書きする（新しい実行のたびにファイルをリセットする）
	var debugWriter *bufio.Writer
	if debugPath != "" {
		f, err := os.Create(debugPath)
		if err != nil {
			return err
		}
		defer f.Close()
		// bufio.Writer でバッファリングし、1 チャンクごとに Flush して確実に書き出す
		debugWriter = bufio.NewWriter(f)
		log.Printf("debug output: %s", debugPath)
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

	// protojson のシリアライズオプション: 改行なしの 1 行 JSON とする
	// protojson のシリアライズオプション: 改行なしの 1 行 JSON とする
	// id フィールドは optional 定義のため、id=0 でも明示的にセットされていれば出力される
	marshaler := protojson.MarshalOptions{Multiline: false}

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

		// --debug が有効な場合、レスポンスを 1 行 JSON としてファイルへ書き出す
		if debugWriter != nil {
			line, err := marshaler.Marshal(resp)
			if err != nil {
				return err
			}
			// 1 行 1 JSON の NDJSON 形式で書き出す（末尾に改行を付与）
			if _, err := debugWriter.Write(line); err != nil {
				return err
			}
			if err := debugWriter.WriteByte('\n'); err != nil {
				return err
			}
			// チャンクごとにフラッシュして途中経過も確実にファイルへ反映する
			if err := debugWriter.Flush(); err != nil {
				return err
			}
		}

		// isFinal = true のチャンクを受信したらストリームの終了と見なして抜ける
		if resp.GetIsFinal() {
			break
		}
	}
	log.Printf("done. total chunks: %d", chunkNum)
	return nil
}
