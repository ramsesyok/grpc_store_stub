package main

import (
	"log"
	"math/rand"
	"os"
	"time"

	simpb "github.com/ramsesyok/grpc-store-stub/gen/simulation"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
)

// genFlags は gen サブコマンドのフラグ値を保持します。
var genFlags struct {
	// output: 出力先のファイルパス
	output string
	// objects: 生成するオブジェクト数
	objects int
	// width / height: エリアのサイズ（x: 0〜width、y: 0〜height）
	width  float64
	height float64
	// rangeStart / rangeEnd: シミュレーション時間の開始・終了（秒）
	rangeStart float64
	rangeEnd   float64
	// interval: シミュレーションの計算間隔（秒）。実時間ではなくシミュレーション上の時間
	interval float64
	// bulkInterval: 1 チャンクあたりのシミュレーション時間の長さ（秒）
	bulkInterval float64
	// wait: チャンク送信間のスリープ時間（秒）。送信レートが高すぎる場合に調整する
	wait float64
}

// genCmd は「gen」サブコマンドの定義です。
// 指定されたパラメータに基づいてシミュレーション用リクエスト JSON を生成し、
// ファイルへ保存します。オブジェクトの初期位置はエリア内でランダムに配置されます。
var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "シミュレーション用リクエスト JSON ファイルを生成する",
	Long: `指定したパラメータでシミュレーション用リクエスト JSON を生成します。
オブジェクトの初期位置はエリア内でランダムに配置されます。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return generateRequest(genFlags)
	},
}

func init() {
	genCmd.Flags().StringVarP(&genFlags.output, "output", "o", "request.json", "出力先のファイルパス")
	genCmd.Flags().IntVarP(&genFlags.objects, "objects", "n", 1, "生成するオブジェクト数")
	genCmd.Flags().Float64VarP(&genFlags.width, "width", "W", 100.0, "エリアの幅（x: 0 〜 width）")
	genCmd.Flags().Float64VarP(&genFlags.height, "height", "H", 100.0, "エリアの高さ（y: 0 〜 height）")
	genCmd.Flags().Float64Var(&genFlags.rangeStart, "range-start", 0.0, "シミュレーション開始時刻（秒）")
	genCmd.Flags().Float64Var(&genFlags.rangeEnd, "range-end", 1000.0, "シミュレーション終了時刻（秒）")
	genCmd.Flags().Float64VarP(&genFlags.interval, "interval", "t", 1.0, "シミュレーションの計算間隔（秒）")
	genCmd.Flags().Float64VarP(&genFlags.bulkInterval, "bulk-interval", "b", 100.0, "1 チャンクあたりのシミュレーション時間の長さ（秒）")
	genCmd.Flags().Float64VarP(&genFlags.wait, "wait", "w", 0.5, "チャンク送信間のスリープ時間（秒）")

	rootCmd.AddCommand(genCmd)
}

// generateRequest はフラグ値をもとにシミュレーション用リクエストを生成し、
// JSON ファイルへ書き出します。
func generateRequest(f struct {
	output       string
	objects      int
	width        float64
	height       float64
	rangeStart   float64
	rangeEnd     float64
	interval     float64
	bulkInterval float64
	wait         float64
}) error {
	// 乱数源を現在時刻で初期化する（実行ごとに異なる配置を生成する）
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 各オブジェクトの初期状態をランダムに生成する
	objs := make([]*simpb.SimObject, f.objects)
	for i := range objs {
		objs[i] = &simpb.SimObject{
			Id: func() *int32 { v := int32(i); return &v }(),
			// x, y はエリア内のランダムな位置
			X: rng.Float64() * f.width,
			Y: rng.Float64() * f.height,
			// z はデータ量調整用。height をスケールとしてランダムな値を設定する
			Z: rng.Float64() * f.height,
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
			XMin: 0, XMax: f.width,
			YMin: 0, YMax: f.height,
		},
		Range:        &simpb.Range{Start: f.rangeStart, End: f.rangeEnd},
		Interval:     f.interval,
		BulkInterval: f.bulkInterval,
		Wait:         f.wait,
	}

	// protojson でインデント付き JSON へシリアライズする
	// フィールド名は proto の命名規則に従い camelCase で出力される（例: bulkInterval）
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(req)
	if err != nil {
		return err
	}

	// JSON をファイルへ書き出す
	if err := os.WriteFile(f.output, data, 0644); err != nil {
		return err
	}
	log.Printf("generated %s (%d objects, area %.0fx%.0f)", f.output, f.objects, f.width, f.height)
	return nil
}
