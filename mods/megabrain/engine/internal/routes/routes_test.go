package routes

import "testing"

func TestEncodeDecodeRoundtrip(t *testing.T) {
	path := [][2]int{{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 2}, {1, 2}}
	x, y, dirs := Encode(path)
	if x != 0 || y != 0 {
		t.Fatalf("start = (%d,%d), want (0,0)", x, y)
	}
	if dirs != "EESSW" {
		t.Fatalf("dirs = %q, want %q", dirs, "EESSW")
	}
	got := Decode(x, y, dirs)
	if len(got) != len(path) {
		t.Fatalf("decoded len = %d, want %d", len(got), len(path))
	}
	for i := range path {
		if got[i] != path[i] {
			t.Fatalf("decoded[%d] = %v, want %v", i, got[i], path[i])
		}
	}
}

func TestHandles(t *testing.T) {
	// Reta: só início e fim.
	straight := [][2]int{{0, 0}, {1, 0}, {2, 0}, {3, 0}}
	if h := Handles(straight); len(h) != 2 || h[0] != 0 || h[1] != 3 {
		t.Fatalf("straight handles = %v, want [0 3]", h)
	}

	// Com curva em (2,0).
	turn := [][2]int{{0, 0}, {1, 0}, {2, 0}, {2, 1}}
	if h := Handles(turn); len(h) != 3 || h[1] != 2 {
		t.Fatalf("turn handles = %v, want [0 2 3]", h)
	}
}

func TestWaypoints(t *testing.T) {
	// Ponto de curva próximo (< 10 tiles): é descartado.
	near := [][2]int{{0, 0}, {1, 0}, {2, 0}, {2, 1}}
	if wps := Waypoints(near); len(wps) != 2 || wps[0] != [2]int{0, 0} || wps[1] != [2]int{2, 1} {
		t.Fatalf("near waypoints = %v, want [(0,0) (2,1)]", wps)
	}

	// Ponto de curva distante (>= 10 tiles): é mantido.
	far := make([][2]int, 0, 12)
	for x := 0; x <= 10; x++ {
		far = append(far, [2]int{x, 0})
	}
	far = append(far, [2]int{10, 1})
	wps := Waypoints(far)
	want := [][2]int{{0, 0}, {10, 0}, {10, 1}}
	if len(wps) != len(want) {
		t.Fatalf("far waypoints = %v, want %v", wps, want)
	}
	for i := range want {
		if wps[i] != want[i] {
			t.Fatalf("far waypoints[%d] = %v, want %v", i, wps[i], want[i])
		}
	}
}
