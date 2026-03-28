// simulation.go はシミュレーションの純粋な演算ロジックです。
// gRPC / protobuf への依存を持たず、単体でテスト・再利用できます。
package main

import "math"

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
// states はインプレースで更新されます。
func step(states []objectState, area simulationArea, interval float64) {
	for i := range states {
		st := &states[i]

		// direction（度数法）をラジアンへ変換して x, y を更新
		rad := st.direction * math.Pi / 180.0
		st.x += st.speed * math.Cos(rad) * interval
		st.y += st.speed * math.Sin(rad) * interval

		// X 境界（垂直壁）: x 成分のみ反転 → new_dir = 180° - dir
		st.x, st.direction = reflectX(st.x, area.xMin, area.xMax, st.direction)
		// Y 境界（水平壁）: y 成分のみ反転 → new_dir = -dir
		st.y, st.direction = reflectY(st.y, area.yMin, area.yMax, st.direction)
	}
}
