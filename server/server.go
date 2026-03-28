// server.go は SimulationService の gRPC 実装です。
// proto 型と演算層（simulation.go）のデータ型を相互変換し、
// ストリームへの送信を担当します。
package main

import (
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
// proto リクエストを演算層の型へ変換してシミュレーションを実行し、
// bulkInterval シミュレーション秒ごとにまとめてクライアントへストリーム送信します。
// 最後のチャンクでは IsFinal = true を設定します。
func (s *simServer) RunSimulation(req *simpb.SimulationRequest, stream simpb.SimulationService_RunSimulationServer) error {
	// interval: シミュレーションの1ステップあたりの時間（秒）
	// 0 以下の場合はデフォルト 1.0 秒とする
	interval := req.GetInterval()
	if interval <= 0 {
		interval = 1.0
	}

	// bulkInterval: 1 回のストリーム送信にまとめるシミュレーション時間の長さ（秒）
	// 0 以下の場合は interval と同じ（1 ステップごとに送信）とする
	bulkInterval := req.GetBulkInterval()
	if bulkInterval <= 0 {
		bulkInterval = interval
	}

	// waitDur: 各チャンク送信後に挿入するスリープ時間
	// 演算が軽すぎて送信が高速になりすぎる場合に速度を調整する目的で使用する
	waitDur := time.Duration(float64(time.Second) * req.GetWait())

	// proto の Area メッセージを演算層の simulationArea へ変換する
	protoArea := req.GetArea()
	area := simulationArea{
		xMin: protoArea.GetXMin(),
		xMax: protoArea.GetXMax(),
		yMin: protoArea.GetYMin(),
		yMax: protoArea.GetYMax(),
	}

	rangeReq := req.GetRange()
	start := rangeReq.GetStart()
	end := rangeReq.GetEnd()

	// proto の SimObject スライスを演算層の objectState スライスへ変換する
	// シミュレーション中はこの状態を step() で直接更新していく
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
	items := make([]*simpb.SimItem, 0)
	// chunkStart は現在送信中チャンクの開始タイムスタンプ（シミュレーション秒）
	chunkStart := start
	// chunkEnd は次の送信タイムスタンプ（chunkStart + bulkInterval）
	chunkEnd := chunkStart + bulkInterval

	// t はシミュレーション上の現在時刻（秒）。interval ずつ進める
	for t := start; t < end; t += interval {
		// 演算層の step() で全オブジェクトを1ステップ分進める
		// simTime にステップ終了時刻を渡し、反射イベントを受け取る
		stepTime := t + interval
		reflections := step(states, area, interval, stepTime)

		// 更新後の objectState スライスを proto の SimAttribute スライスへ変換する
		attrs := make([]*simpb.SimAttribute, 0, len(states))
		for _, st := range states {
			attrs = append(attrs, &simpb.SimAttribute{
				Id:        &st.id,
				X:         st.x,
				Y:         st.y,
				Z:         st.z, // 変化なし（パススルー）
				Direction: st.direction,
				Speed:     st.speed,
			})
		}

		// 反射イベントを proto の SimEvent スライスへ変換する
		// args の値型は MessageArg の oneof で表現する
		simEvents := make([]*simpb.SimEvent, 0, len(reflections))
		for _, r := range reflections {
			simEvents = append(simEvents, &simpb.SimEvent{
				Type: "m0001",
				Args: map[string]*simpb.MessageArg{
					"id": {Kind: &simpb.MessageArg_IntValue{IntValue: int64(r.objectID)}},
					"x":  {Kind: &simpb.MessageArg_DoubleValue{DoubleValue: r.x}},
					"y":  {Kind: &simpb.MessageArg_DoubleValue{DoubleValue: r.y}},
					"t":  {Kind: &simpb.MessageArg_StringValue{StringValue: formatSimTime(r.simTime)}},
				},
			})
		}

		// このステップの結果を SimItem としてバッファへ追加
		// Timestamp はステップ終了時刻（t + interval）とする
		items = append(items, &simpb.SimItem{
			Timestamp:  stepTime,
			Attributes: attrs,
			Events:     simEvents,
		})

		// ステップ終了時刻が chunkEnd に達したらストリームへ送信する
		stepEnd := t + interval
		if stepEnd >= chunkEnd {
			isFinal := stepEnd >= end
			resp := &simpb.SimulationResponse{
				Items:     items,
				ItemCount: int32(len(items)),
				Range:     &simpb.Range{Start: chunkStart, End: stepEnd},
				IsFinal:   isFinal,
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
			// 送信レートを調整するためにスリープを挿入する（wait > 0 の場合のみ）
			if waitDur > 0 {
				time.Sleep(waitDur)
			}
			// 次チャンクの範囲へ更新してバッファをリセット
			chunkStart = stepEnd
			chunkEnd = chunkStart + bulkInterval
			items = make([]*simpb.SimItem, 0)
			if isFinal {
				return nil
			}
		}
	}

	// ループ終了後、bulkInterval に満たない残りのアイテムをまとめて送信する
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
