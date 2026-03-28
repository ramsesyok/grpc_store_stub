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

// reflect はオブジェクトがエリア境界に達したときの反射処理を行います。
// pos が [min, max] の範囲を超えた場合、境界で折り返し、
// dir = π - dir で速度の軸成分を反転させます。
// 戻り値は補正後の位置と方向（度数法）です。
func reflect(pos, min, max, dir float64) (float64, float64) {
	if pos < min {
		// 下限を下回った場合：はみ出た分だけ min から折り返す
		pos = min + (min - pos)
		dir = math.Pi - dir
	} else if pos > max {
		// 上限を超えた場合：はみ出た分だけ max から折り返す
		pos = max - (pos - max)
		dir = math.Pi - dir
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

		// エリア境界に達した場合は反射させる（x, y それぞれ独立）
		st.x, st.direction = reflect(st.x, area.xMin, area.xMax, st.direction)
		st.y, st.direction = reflect(st.y, area.yMin, area.yMax, st.direction)
	}
}
