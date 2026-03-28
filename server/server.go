package main

import (
	"math"
	"time"

	simpb "github.com/ramsesyok/grpc-store-stub/gen/simulation"
)

// simServer は SimulationService の実装です。
// UnimplementedSimulationServiceServer を埋め込むことで、
// 将来 RPC が追加された場合もコンパイルエラーを防ぎます。
type simServer struct {
	simpb.UnimplementedSimulationServiceServer
}

// RunSimulation は SimulationService の server streaming RPC 実装です。
// リクエストで指定された条件に従ってシミュレーションを実行し、
// bulkSize ステップごとにまとめてクライアントへストリーム送信します。
// 最後のチャンクでは IsFinal = true を設定します。
func (s *simServer) RunSimulation(req *simpb.SimulationRequest, stream simpb.SimulationService_RunSimulationServer) error {
	// interval: シミュレーションの1ステップあたりの時間（秒）
	// 0 以下の場合はデフォルト 1.0 秒とする
	interval := req.GetInterval()
	if interval <= 0 {
		interval = 1.0
	}

	// bulkSize: 1 回のストリーム送信にまとめるステップ数
	// 0 以下の場合は最低 1 に補正する
	bulkSize := int(req.GetBulkSize())
	if bulkSize <= 0 {
		bulkSize = 1
	}

	// waitDur: 各チャンク送信後に挿入するスリープ時間
	// 演算が軽すぎて送信が高速になりすぎる場合に速度を調整する目的で使用する
	waitDur := time.Duration(float64(time.Second) * req.GetWait())

	area := req.GetArea()
	rangeReq := req.GetRange()
	start := rangeReq.GetStart()
	end := rangeReq.GetEnd()

	// リクエストの objects を内部状態 objectState のスライスへコピーする
	// シミュレーション中はこの状態を直接更新していく
	states := make([]objectState, 0, len(req.GetObjects()))
	for _, o := range req.GetObjects() {
		states = append(states, objectState{
			id:        o.GetId(),
			x:         o.GetX(),
			y:         o.GetY(),
			z:         o.GetZ(), // 演算には使わないがレスポンスにパススルー
			direction: o.GetDirection(),
			speed:     o.GetSpeed(),
		})
	}

	// items は次の送信タイミングまで蓄積する SimItem のバッファ
	items := make([]*simpb.SimItem, 0, bulkSize)
	// chunkStart は現在送信中チャンクの開始タイムスタンプ
	chunkStart := start

	// t はシミュレーション上の現在時刻（秒）。interval ずつ進める
	for t := start; t < end; t += interval {
		// --- 1 ステップ分の全オブジェクト位置更新 ---
		attrs := make([]*simpb.SimAttribute, 0, len(states))
		for i := range states {
			st := &states[i]

			// direction（度数法）をラジアンへ変換して x, y を更新
			rad := st.direction * math.Pi / 180.0
			st.x += st.speed * math.Cos(rad) * interval
			st.y += st.speed * math.Sin(rad) * interval

			// エリア境界に達した場合は反射させる（x, y それぞれ独立）
			st.x, st.direction = reflect(st.x, area.GetXMin(), area.GetXMax(), st.direction)
			st.y, st.direction = reflect(st.y, area.GetYMin(), area.GetYMax(), st.direction)

			attrs = append(attrs, &simpb.SimAttribute{
				Id:        st.id,
				X:         st.x,
				Y:         st.y,
				Z:         st.z, // 変化なし（パススルー）
				Direction: st.direction,
				Speed:     st.speed,
			})
		}

		// このステップの結果を SimItem としてバッファへ追加
		// Timestamp はステップ終了時刻（t + interval）とする
		// Events は現時点では空リストで送信する
		items = append(items, &simpb.SimItem{
			Timestamp:  t + interval,
			Attributes: attrs,
			Events:     []*simpb.SimEvent{},
		})

		// bulkSize 分のステップが蓄積されたらストリームへ送信する
		if len(items) >= bulkSize {
			chunkEnd := t + interval
			// このチャンクがシミュレーション終端に達したか判定
			isFinal := chunkEnd >= end
			resp := &simpb.SimulationResponse{
				Items:     items,
				ItemCount: int32(len(items)),
				Range:     &simpb.Range{Start: chunkStart, End: chunkEnd},
				IsFinal:   isFinal,
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
			// 送信レートを調整するためにスリープを挿入する（wait > 0 の場合のみ）
			if waitDur > 0 {
				time.Sleep(waitDur)
			}
			// 次チャンクの開始タイムスタンプを更新してバッファをリセット
			chunkStart = chunkEnd
			items = make([]*simpb.SimItem, 0, bulkSize)
			if isFinal {
				return nil
			}
		}
	}

	// ループ終了後、bulkSize に満たない残りのアイテムをまとめて送信する
	// このチャンクが必ず最後になるため IsFinal = true を設定する
	if len(items) > 0 {
		resp := &simpb.SimulationResponse{
			Items:     items,
			ItemCount: int32(len(items)),
			Range:     &simpb.Range{Start: chunkStart, End: end},
			IsFinal:   true,
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}
