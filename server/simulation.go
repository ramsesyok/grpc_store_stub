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

// reflect はオブジェクトがエリア境界に達したときの反射処理を行います。
// pos が [min, max] の範囲を超えた場合、境界で折り返し、
// X 軸方向の反射では dir = π - dir（Y 方向の速度成分を反転）します。
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
