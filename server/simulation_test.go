package main

import (
	"math"
	"testing"
)

const eps = 1e-9

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < eps
}

// ============================================================
// reflect のテスト
// ============================================================

func TestReflectX(t *testing.T) {
	tests := []struct {
		name             string
		pos, min, max    float64
		dir              float64
		wantPos, wantDir float64
	}{
		{
			name:    "範囲内のため変化なし",
			pos: 50, min: 0, max: 100, dir: 45,
			wantPos: 50, wantDir: 45,
		},
		{
			name:    "最小値ちょうどは変化なし",
			pos: 0, min: 0, max: 100, dir: 45,
			wantPos: 0, wantDir: 45,
		},
		{
			name:    "最大値ちょうどは変化なし",
			pos: 100, min: 0, max: 100, dir: 45,
			wantPos: 100, wantDir: 45,
		},
		{
			// 左壁反射: 垂直壁で x 成分が反転する → new_dir = 180 - dir
			// dir=225°（南西）→ new_dir = 180 - 225 = -45°（南東）
			name:    "左壁（最小値）を超えて反射",
			pos: -5, min: 0, max: 100, dir: 225,
			wantPos: 5, wantDir: 180 - 225, // = -45
		},
		{
			// 右壁反射: dir=45°（北東）→ new_dir = 180 - 45 = 135°（北西）
			name:    "右壁（最大値）を超えて反射",
			pos: 103, min: 0, max: 100, dir: 45,
			wantPos: 97, wantDir: 135,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPos, gotDir := reflectX(tt.pos, tt.min, tt.max, tt.dir)
			if !almostEqual(gotPos, tt.wantPos) {
				t.Errorf("pos: got %.6f, want %.6f", gotPos, tt.wantPos)
			}
			if !almostEqual(gotDir, tt.wantDir) {
				t.Errorf("dir: got %.6f, want %.6f", gotDir, tt.wantDir)
			}
		})
	}
}

func TestReflectY(t *testing.T) {
	tests := []struct {
		name             string
		pos, min, max    float64
		dir              float64
		wantPos, wantDir float64
	}{
		{
			name:    "範囲内のため変化なし",
			pos: 50, min: 0, max: 100, dir: 45,
			wantPos: 50, wantDir: 45,
		},
		{
			// 下壁反射: 水平壁で y 成分が反転する → new_dir = -dir
			// dir=270°（南）→ new_dir = -270°（= 90°相当、北）
			name:    "下壁（最小値）を超えて反射",
			pos: -3, min: 0, max: 100, dir: 270,
			wantPos: 3, wantDir: -270,
		},
		{
			// 上壁反射: dir=90°（北）→ new_dir = -90°（= 270°相当、南）
			name:    "上壁（最大値）を超えて反射",
			pos: 105, min: 0, max: 100, dir: 90,
			wantPos: 95, wantDir: -90,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPos, gotDir := reflectY(tt.pos, tt.min, tt.max, tt.dir)
			if !almostEqual(gotPos, tt.wantPos) {
				t.Errorf("pos: got %.6f, want %.6f", gotPos, tt.wantPos)
			}
			if !almostEqual(gotDir, tt.wantDir) {
				t.Errorf("dir: got %.6f, want %.6f", gotDir, tt.wantDir)
			}
		})
	}
}

// ============================================================
// step のテスト
// ============================================================

func TestStep(t *testing.T) {
	area := simulationArea{xMin: 0, xMax: 100, yMin: 0, yMax: 100}

	t.Run("東方向（dir=0°）に直進、反射なし", func(t *testing.T) {
		states := []objectState{{id: 0, x: 10, y: 50, z: 0, direction: 0, speed: 5}}
		events := step(states, area, 1.0, 1.0)
		// Δx = 5*cos(0°)*1 = 5、Δy = 5*sin(0°)*1 = 0
		wantX, wantY, wantDir := 15.0, 50.0, 0.0
		checkState(t, states[0], wantX, wantY, wantDir)
		if len(events) != 0 {
			t.Errorf("events: got %d, want 0", len(events))
		}
	})

	t.Run("北方向（dir=90°）に直進、反射なし", func(t *testing.T) {
		states := []objectState{{id: 0, x: 50, y: 10, z: 0, direction: 90, speed: 5}}
		events := step(states, area, 1.0, 1.0)
		// Δx = 5*cos(90°)*1 ≈ 0、Δy = 5*sin(90°)*1 = 5
		wantX, wantY, wantDir := 50.0, 15.0, 90.0
		checkState(t, states[0], wantX, wantY, wantDir)
		if len(events) != 0 {
			t.Errorf("events: got %d, want 0", len(events))
		}
	})

	t.Run("東方向、右壁に当たり反射", func(t *testing.T) {
		// x=98、dir=0°、speed=5 → x=103 → 右壁反射
		states := []objectState{{id: 0, x: 98, y: 50, z: 0, direction: 0, speed: 5}}
		events := step(states, area, 1.0, 10.0)
		// new_x = 100 - (103-100) = 97、new_dir = 180 - 0 = 180°
		wantX, wantY, wantDir := 97.0, 50.0, 180.0
		checkState(t, states[0], wantX, wantY, wantDir)
		// 反射イベントが 1 件発生していること
		if len(events) != 1 {
			t.Fatalf("events: got %d, want 1", len(events))
		}
		// イベントの衝突位置は右壁（x=100）、y は変化なし
		if !almostEqual(events[0].x, 100.0) {
			t.Errorf("event.x: got %.6f, want 100.0", events[0].x)
		}
		if !almostEqual(events[0].y, 50.0) {
			t.Errorf("event.y: got %.6f, want 50.0", events[0].y)
		}
		if events[0].objectID != 0 {
			t.Errorf("event.objectID: got %d, want 0", events[0].objectID)
		}
		if events[0].simTime != 10.0 {
			t.Errorf("event.simTime: got %.1f, want 10.0", events[0].simTime)
		}
	})

	t.Run("北方向、上壁に当たり反射", func(t *testing.T) {
		// y=97、dir=90°、speed=5 → y=102 → 上壁反射
		states := []objectState{{id: 0, x: 50, y: 97, z: 0, direction: 90, speed: 5}}
		events := step(states, area, 1.0, 20.0)
		// new_y = 100 - (102-100) = 98、new_dir = -90°
		wantX, wantY, wantDir := 50.0, 98.0, -90.0
		checkState(t, states[0], wantX, wantY, wantDir)
		// 反射イベントが 1 件発生していること
		if len(events) != 1 {
			t.Fatalf("events: got %d, want 1", len(events))
		}
		// イベントの衝突位置は上壁（y=100）、x は変化なし
		if !almostEqual(events[0].x, 50.0) {
			t.Errorf("event.x: got %.6f, want 50.0", events[0].x)
		}
		if !almostEqual(events[0].y, 100.0) {
			t.Errorf("event.y: got %.6f, want 100.0", events[0].y)
		}
	})

	t.Run("北東方向（dir=45°）、右壁に当たり反射", func(t *testing.T) {
		// Δ = 5 * √2/2 ≈ 3.5355
		// x=97+Δ≈100.5355 → 右壁反射: new_x=99.4645、new_dir=135°
		// y=50+Δ≈53.5355 → 範囲内
		delta := 5 * math.Sqrt2 / 2
		states := []objectState{{id: 0, x: 97, y: 50, z: 0, direction: 45, speed: 5}}
		events := step(states, area, 1.0, 5.0)
		wantX := 100 - (97 + delta - 100) // = 103 - delta
		wantY := 50 + delta
		wantDir := 135.0
		checkState(t, states[0], wantX, wantY, wantDir)
		// X 壁のみ反射
		if len(events) != 1 {
			t.Fatalf("events: got %d, want 1", len(events))
		}
		if !almostEqual(events[0].x, 100.0) {
			t.Errorf("event.x: got %.6f, want 100.0", events[0].x)
		}
	})

	t.Run("z はパススルーで変化なし", func(t *testing.T) {
		states := []objectState{{id: 0, x: 10, y: 10, z: 42, direction: 0, speed: 5}}
		step(states, area, 1.0, 1.0)
		if !almostEqual(states[0].z, 42) {
			t.Errorf("z: got %.6f, want 42", states[0].z)
		}
	})

	t.Run("複数オブジェクトが独立して処理される", func(t *testing.T) {
		states := []objectState{
			{id: 0, x: 98, y: 50, z: 0, direction: 0, speed: 5}, // 右壁で反射
			{id: 1, x: 10, y: 50, z: 0, direction: 0, speed: 5}, // 直進
		}
		events := step(states, area, 1.0, 1.0)
		// obj0: x=97, dir=180°
		checkState(t, states[0], 97.0, 50.0, 180.0)
		// obj1: x=15, dir=0°
		checkState(t, states[1], 15.0, 50.0, 0.0)
		// obj0 のみ反射イベント
		if len(events) != 1 {
			t.Fatalf("events: got %d, want 1", len(events))
		}
		if events[0].objectID != 0 {
			t.Errorf("event.objectID: got %d, want 0", events[0].objectID)
		}
	})

	t.Run("反射後の方向でも x, y が正しく更新される（2ステップ目の検証）", func(t *testing.T) {
		// 1ステップ目: 東方向、右壁反射 → dir=180°
		states := []objectState{{id: 0, x: 98, y: 50, z: 0, direction: 0, speed: 5}}
		step(states, area, 1.0, 1.0) // → x=97, dir=180°

		// 2ステップ目: 西方向（180°）に移動
		events := step(states, area, 1.0, 2.0)
		// Δx = 5*cos(180°)*1 = -5 → x = 97-5 = 92
		wantX, wantY, wantDir := 92.0, 50.0, 180.0
		checkState(t, states[0], wantX, wantY, wantDir)
		if len(events) != 0 {
			t.Errorf("2ステップ目は反射なし: got %d events", len(events))
		}
	})
}

// ============================================================
// formatSimTime のテスト
// ============================================================

func TestFormatSimTime(t *testing.T) {
	tests := []struct {
		simTime float64
		want    string
	}{
		{0, "00:00:00"},
		{59, "00:00:59"},
		{60, "00:01:00"},
		{3599, "00:59:59"},
		{3600, "01:00:00"},
		{3661, "01:01:01"},
		{3723.9, "01:02:03"}, // 小数部分は切り捨て
		{86399, "23:59:59"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatSimTime(tt.simTime)
			if got != tt.want {
				t.Errorf("formatSimTime(%.1f): got %q, want %q", tt.simTime, got, tt.want)
			}
		})
	}
}

// checkState は objectState の x, y, direction を検証するヘルパーです。
func checkState(t *testing.T, st objectState, wantX, wantY, wantDir float64) {
	t.Helper()
	if !almostEqual(st.x, wantX) {
		t.Errorf("x: got %.6f, want %.6f", st.x, wantX)
	}
	if !almostEqual(st.y, wantY) {
		t.Errorf("y: got %.6f, want %.6f", st.y, wantY)
	}
	if !almostEqual(st.direction, wantDir) {
		t.Errorf("direction: got %.6f, want %.6f", st.direction, wantDir)
	}
}
