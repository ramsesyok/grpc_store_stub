// Package main は SimulationService の gRPC クライアントです。
// cobra を使用して以下のサブコマンドを提供します。
//
//	run  JSONファイルを読み込んでサーバーへリクエストを送信し、ストリームを受信する
//	gen  シミュレーション用リクエスト JSON ファイルを生成する
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd はクライアントのルートコマンドです。
// サブコマンド（run / gen）の親となります。
var rootCmd = &cobra.Command{
	Use:   "client",
	Short: "SimulationService gRPC クライアント",
	Long: `SimulationService gRPC クライアント

サブコマンド:
  run  リクエスト JSON をもとにシミュレーションを実行し、結果を受信します
  gen  シミュレーション用リクエスト JSON ファイルを生成します`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
