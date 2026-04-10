# grpc-store-stub

等速直線運動するオブジェクト群のシミュレーションを行う gRPC スタブです。
大量のレスポンスデータを模擬することで、gRPC の server streaming 実装の検証に使用します。

## 概要

リクエストでシミュレーション条件（オブジェクト数・エリア・時間範囲など）を指定すると、
サーバーがシミュレーションを実行し、結果を一定ステップ数ごとにまとめてストリーム送信します。
最後のチャンクで `isFinal = true` が設定され、ストリームが終了します。

```
Client                          Server
  |                               |
  |-- RunSimulation(request) ---> |
  |                               | シミュレーション実行
  |<-- SimulationResponse --------|  (bulkSize ステップ分)
  |<-- SimulationResponse --------|
  |         ...                   |
  |<-- SimulationResponse --------|  isFinal = true
  |                               |
```

## ディレクトリ構成

```
grpc-store-stub/
├── proto/
│   └── simulation/
│       └── simulation.proto       # サービス・メッセージ定義
├── gen/
│   └── simulation/
│       ├── simulation.pb.go       # protoc 生成コード（メッセージ）
│       └── simulation_grpc.pb.go  # protoc 生成コード（サービス）
├── server/
│   ├── main.go                    # サーバー起動
│   ├── server.go                  # RPC 実装（gRPC 層）
│   ├── job_manager.go             # ジョブキャンセル管理
│   └── simulation.go              # シミュレーション演算ロジック
├── client/
│   ├── main.go                    # エントリポイント・サブコマンド振り分け
│   ├── cmd_run.go                 # run サブコマンド
│   └── cmd_gen.go                 # gen サブコマンド
├── Makefile
├── sample_request.json            # リクエストのサンプル
└── sample_response.json           # レスポンスのサンプル
```

## セットアップ

### 必要ツール

| ツール | 用途 |
|---|---|
| Go 1.21 以上 | ビルド・実行 |
| protoc | .proto ファイルからコード生成 |
| protoc-gen-go | Go 向けメッセージコード生成プラグイン |
| protoc-gen-go-grpc | Go 向けサービスコード生成プラグイン |

```bash
# protoc プラグインのインストール
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### ビルド

```bash
# 1. proto ファイルから Go コードを生成する
make proto

# 2. 依存関係を解決する
make tidy

# 3. サーバー・クライアントをビルドする
make build
# → bin/stub-server, bin/support-client が生成される
```

## 使い方

### サーバーの起動

```bash
make run-server
# または
./bin/stub-server
# → 2024/xx/xx server listening on :50051
```

### クライアント: リクエスト JSON の生成 (`gen`)

シミュレーション用リクエスト JSON ファイルを生成します。
オブジェクトの初期位置はエリア内でランダムに配置されます。

```bash
./bin/support-client gen [flags]
```

| フラグ | 短縮 | デフォルト | 説明 |
|---|---|---|---|
| `--output` | `-o` | `request.json` | 出力ファイルパス |
| `--objects` | `-n` | `1` | オブジェクト数 |
| `--width` | `-W` | `100` | エリア幅（x: 0 〜 width）|
| `--height` | `-H` | `100` | エリア高さ（y: 0 〜 height）|
| `--range-start` | | `0` | シミュレーション開始時刻（秒）|
| `--range-end` | | `1000` | シミュレーション終了時刻（秒）|
| `--interval` | `-t` | `1.0` | 計算間隔（秒）|
| `--bulk-interval` | `-b` | `100` | 1 チャンクあたりのシミュレーション時間の長さ（秒）|
| `--wait` | `-w` | `0.5` | チャンク間のスリープ時間（秒）|
| `--job-id` | | `jobId` | ジョブの識別子（キャンセル時に使用）|

```bash
# 例: オブジェクト 10 個、エリア 200x150 で生成
./bin/support-client gen -n 10 -W 200 -H 150 -o request.json
```

### クライアント: シミュレーションの実行 (`run`)

JSON ファイルをリクエストとしてサーバーへ送信し、ストリームを受信します。

```bash
./bin/support-client run [flags]
```

| フラグ | 短縮 | デフォルト | 説明 |
|---|---|---|---|
| `--input` | `-i` | `request.json` | リクエスト JSON ファイルパス |
| `--server` | `-s` | `localhost:50051` | gRPC サーバーアドレス |

```bash
# 例: 生成したファイルを使ってシミュレーション実行
./bin/support-client run -i request.json

# 例: 別ホストのサーバーへ接続
./bin/support-client run -i request.json -s 192.168.1.10:50051
```

### 実行例

```bash
# ターミナル 1: サーバー起動
./bin/stub-server

# ターミナル 2: リクエスト生成 → 実行
./bin/support-client gen -n 5 -W 300 -H 200 --range-end 5000 --bulk-size 200 -o req.json
./bin/support-client run -i req.json

# 出力例:
# 2024/xx/xx [chunk 1] itemCount=200 range=[0.00, 200.00] isFinal=false
# 2024/xx/xx [chunk 2] itemCount=200 range=[200.00, 400.00] isFinal=false
# ...
# 2024/xx/xx [chunk 25] itemCount=200 range=[4800.00, 5000.00] isFinal=true
# 2024/xx/xx done. total chunks: 25
```

既存の `sample_request.json` もそのまま利用できます。

```bash
./bin/support-client run -i sample_request.json
```

## gRPC インタフェース

### サービス定義

```protobuf
service SimulationService {
  rpc RunSimulation(SimulationRequest) returns (stream SimulationResponse);
  rpc CancelSimulation(CancelRequest) returns (CancelResponse);
}
```

### リクエスト (`SimulationRequest`)

| フィールド | 型 | 説明 |
|---|---|---|
| `objects` | `SimObject[]` | オブジェクト一覧（初期位置・速度・方向）|
| `area` | `Area` | 移動エリアの範囲（x_min/x_max/y_min/y_max）|
| `range` | `Range` | シミュレーション時間の開始・終了（秒）|
| `interval` | `double` | 計算間隔（秒）。実時間ではなくシミュレーション上の時間 |
| `bulk_interval` | `double` | 1 チャンクあたりのシミュレーション時間の長さ（秒）|
| `wait` | `double` | チャンク送信間のスリープ時間（秒）|
| `job_id` | `string` | ジョブの識別子。`CancelSimulation` でキャンセルする際に使用する |

`SimObject` のフィールド: `id`, `x`, `y`, `z`, `direction`（度数法）, `speed`

### レスポンス (`SimulationResponse`)

| フィールド | 型 | 説明 |
|---|---|---|
| `items` | `SimItem[]` | ステップごとの結果 |
| `item_count` | `int32` | `items` の件数 |
| `range` | `Range` | このチャンクが表す時間範囲 |
| `is_final` | `bool` | 最終チャンクかどうか |
| `job_id` | `string` | リクエストの `job_id` をそのまま返す |
| `server_id` | `string` | レスポンスを返したサーバーの識別子（環境変数 `SERVER_ID`、未設定時は端末 IP:ポート）|

`SimItem` のフィールド: `timestamp`, `attributes`（各オブジェクトの状態）, `events`（現時点は空）

### キャンセル (`CancelSimulation`)

実行中の `RunSimulation` ジョブをキャンセルします。
キャンセルはチャンク送信のタイミング（ステップ単位）で検知されます。
キャンセルが検知されると、その時点までのバッファを `is_final = true` で送信してストリームを終了します。

#### リクエスト (`CancelRequest`)

| フィールド | 型 | 説明 |
|---|---|---|
| `job_id` | `string` | キャンセルするジョブの識別子 |

#### レスポンス (`CancelResponse`)

| フィールド | 型 | 説明 |
|---|---|---|
| `success` | `bool` | キャンセルに成功した場合 `true`。ジョブが存在しない場合は `false` |
| `message` | `string` | 結果メッセージ |

#### サーバーの識別子 (`server_id`)

サーバー起動時に環境変数 `SERVER_ID` を参照します。未設定の場合は `端末IPアドレス:50051` が自動設定されます。
複数サーバーを並行稼働させる場合に `SERVER_ID` でどのサーバーが応答したかを識別できます。

```bash
SERVER_ID=server-1 ./bin/stub-server
```

### `bulk_interval` と `interval` の関係

```
bulk_interval=100 の場合、シミュレーション時間 100 秒ごとに 1 チャンクを送信:
  range=[0,100], range=[100,200], ... という順に届く

interval=1,   bulk_interval=100 → 100 ステップで 1 チャンク (itemCount=100)
interval=0.5, bulk_interval=100 → 200 ステップで 1 チャンク (itemCount=200)
```

### `events` と `MessageArg`

`events` は多言語メッセージ出力のための構造を持ちます（現時点では空リスト固定）。
受信側は `type` をキーに言語ファイルを引き、`args` でプレースホルダを置換します。

```json
// イベントの例
{ "type": "m0001", "args": { "name": "Jane", "age": 17, "height": 163.5 } }
// 言語ファイル: "m0001": "Hello, {name}. I am {age} years old."
// → "Hello, Jane. I am 17 years old."
```

`args` の値は `int64` / `double` / `string` の oneof で表現されます。

## シミュレーションのアルゴリズム

各オブジェクトは `interval` 秒ごとに以下の演算を行います。

1. **移動**: `direction`（度数法）をラジアンに変換し、`speed` × `interval` 分だけ x, y を更新
2. **反射**: x, y がエリア範囲を超えた場合、境界で折り返す
   - 垂直壁（x 境界）: `new_direction = 180° - direction`
   - 水平壁（y 境界）: `new_direction = -direction`
3. **z**: 演算対象外。リクエストの値をそのままレスポンスに返す

## Makefile ターゲット

| コマンド | 説明 |
|---|---|
| `make proto` | proto から Go コードを生成 |
| `make tidy` | `go mod tidy` を実行 |
| `make build` | `bin/stub-server`, `bin/support-client` をビルド |
| `make dist` | 配布用 ZIP アーカイブを生成（下記参照）|
| `make run-server` | サーバーを起動 |
| `make run-client` | クライアントを実行（`request.json` を使用）|
| `make clean` | `bin/`, `dist/` を削除 |

### `make dist` — 配布用アーカイブの生成

指定した OS・アーキテクチャ向けにクロスビルドし、バイナリを ZIP に固めます。

```bash
make dist [OS=<os>] [ARCH=<arch>] [VERSION=<version>]
```

| 変数 | デフォルト | 説明 |
|---|---|---|
| `OS` | `linux` | ターゲット OS（`linux` / `windows` / `darwin`）|
| `ARCH` | `amd64` | ターゲットアーキテクチャ（`amd64` / `arm64` など）|
| `VERSION` | `0.0.0` | バージョン文字列（ZIP ファイル名に使用）|

生成される ZIP ファイルは `dist/grpc-store-stub-<VERSION>-<OS>-<ARCH>.zip` です。
ZIP にはビルドされた `stub-server` と `support-client`（Windows の場合は `.exe` 付き）が含まれます。

```bash
# 例: Windows 向け v1.0.0 をビルド
make dist OS=windows ARCH=amd64 VERSION=1.0.0
# → dist/grpc-store-stub-1.0.0-windows-amd64.zip

# 例: macOS (Apple Silicon) 向け
make dist OS=darwin ARCH=arm64 VERSION=1.0.0
# → dist/grpc-store-stub-1.0.0-darwin-arm64.zip
```
