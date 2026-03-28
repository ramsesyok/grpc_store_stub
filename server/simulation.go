// simulation.go はシミュレーションの純粋な演算ロジックです。
// gRPC / protobuf への依存を持たず、単体でテスト・再利用できます。
package main

import (
	"fmt"
	"math"
)

// objectState はシミュレーション中の1オブジェクトの状態を保持します。
// direction は度数法（0〜360）で管理し、演算時にラジアンへ変換します。
// z はリクエストからパススルーするだけで演算には使用しません。
type objectState struct {
	id        int32
	x, y, z   float64
	direction float64
	speed     float64
}

// simulationArea はオブジェクトが移動できるエリアの範囲を表します。
// gRPC の Area メッセージに対応する純粋なデータ構造です。
type simulationArea struct {
	xMin, xMax float64
	yMin, yMax float64
}

// reflectionEvent は反射発生時のイベントデータです（gRPC 非依存）。
// server.go で SimEvent へ変換されます。
type reflectionEvent struct {
	// 反射したオブジェクトの ID
	objectID int32
	// 壁に当たった位置（x: X 境界の場合は境界値、y: Y 境界の場合は境界値）
	x, y float64
	// 反射が発生したシミュレーション時刻（秒）
	simTime float64
}

// formatSimTime はシミュレーション時間（秒）を "HH:MM:SS" 形式の文字列に変換します。
// 小数部分は切り捨てて整数秒として扱います。
func formatSimTime(t float64) string {
	total := int(t)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// reflectX はオブジェクトが X 軸方向の境界（垂直壁）に達したときの反射処理です。
// 垂直壁では x 成分の速度のみが反転するため、新方向 = 180° - dir となります。
// 戻り値は補正後の位置と方向（度数法）です。
func reflectX(pos, min, max, dir float64) (float64, float64) {
	if pos < min {
		// 下限を下回った場合：はみ出た分だけ min から折り返す
		pos = min + (min - pos)
		dir = 180.0 - dir
	} else if pos > max {
		// 上限を超えた場合：はみ出た分だけ max から折り返す
		pos = max - (pos - max)
		dir = 180.0 - dir
	}
	return pos, dir
}

// reflectY はオブジェクトが Y 軸方向の境界（水平壁）に達したときの反射処理です。
// 水平壁では y 成分の速度のみが反転するため、新方向 = -dir となります。
// 戻り値は補正後の位置と方向（度数法）です。
func reflectY(pos, min, max, dir float64) (float64, float64) {
	if pos < min {
		// 下限を下回った場合：はみ出た分だけ min から折り返す
		pos = min + (min - pos)
		dir = -dir
	} else if pos > max {
		// 上限を超えた場合：はみ出た分だけ max から折り返す
		pos = max - (pos - max)
		dir = -dir
	}
	return pos, dir
}

// step は全オブジェクトを1ステップ（interval 秒）分進めます。
// 等速直線運動で x, y を更新し、エリア境界に達した場合は反射させます。
// simTime はこのステップ終了時のシミュレーション時刻（秒）で、イベントのタイムスタンプに使います。
// states はインプレースで更新され、反射が発生した場合は reflectionEvent のスライスを返します。
func step(states []objectState, area simulationArea, interval float64, simTime float64) []reflectionEvent {
	var events []reflectionEvent

	for i := range states {
		st := &states[i]

		// direction（度数法）をラジアンへ変換して移動後の座標を計算する
		rad := st.direction * math.Pi / 180.0
		newX := st.x + st.speed*math.Cos(rad)*interval
		newY := st.y + st.speed*math.Sin(rad)*interval

		// X 境界（垂直壁）の反射検出: 境界値を衝突位置の x とする
		if newX < area.xMin {
			events = append(events, reflectionEvent{
				objectID: st.id,
				x:        area.xMin,
				y:        newY, // x 反射時の y はまだ未補正の移動後座標
				simTime:  simTime,
			})
		} else if newX > area.xMax {
			events = append(events, reflectionEvent{
				objectID: st.id,
				x:        area.xMax,
				y:        newY,
				simTime:  simTime,
			})
		}
		st.x, st.direction = reflectX(newX, area.xMin, area.xMax, st.direction)

		// Y 境界（水平壁）の反射検出: 境界値を衝突位置の y とし、x は x 反射後の値を使う
		if newY < area.yMin {
			events = append(events, reflectionEvent{
				objectID: st.id,
				x:        st.x,
				y:        area.yMin,
				simTime:  simTime,
			})
		} else if newY > area.yMax {
			events = append(events, reflectionEvent{
				objectID: st.id,
				x:        st.x,
				y:        area.yMax,
				simTime:  simTime,
			})
		}
		st.y, st.direction = reflectY(newY, area.yMin, area.yMax, st.direction)
	}

	return events
}
