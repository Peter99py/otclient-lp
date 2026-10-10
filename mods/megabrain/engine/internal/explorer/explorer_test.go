package explorer

import "testing"

// TestCoverageRouteCoversAll garante que a rota visita todos os tiles andáveis
// e que cada passo é ortogonalmente adjacente.
func TestCoverageRouteCoversAll(t *testing.T) {
	walkable := map[[2]int]bool{
		{0, 0}: true, {1, 0}: true, {2, 0}: true,
		{0, 1}: true, {1, 1}: true,
		{1, 2}: true,
	}
	path := CoverageRoute(walkable, [2]int{0, 0}, 1)
	if len(path) == 0 {
		t.Fatal("empty path")
	}
	if path[0] != [2]int{0, 0} {
		t.Fatalf("path[0] = %v, want (0,0)", path[0])
	}
	if path[len(path)-1] != [2]int{0, 0} {
		t.Fatalf("path does not return to start: %v", path[len(path)-1])
	}

	visited := map[[2]int]bool{}
	for i, p := range path {
		if !walkable[p] {
			t.Fatalf("path[%d] = %v is not walkable", i, p)
		}
		visited[p] = true
		if i > 0 {
			dx := p[0] - path[i-1][0]
			dy := p[1] - path[i-1][1]
			if abs(dx)+abs(dy) != 1 {
				t.Fatalf("step %d->%d not adjacent: %v -> %v", i-1, i, path[i-1], p)
			}
		}
	}
	for tile := range walkable {
		if !visited[tile] {
			t.Fatalf("tile %v not visited", tile)
		}
	}
}

// TestCoverageRouteEfficient garante que a cobertura evita backtracking
// desnecessário: numa sala 3x3 o DFS antigo fazia 17 passos; o novo deve ficar
// bem abaixo disso.
func TestCoverageRouteEfficient(t *testing.T) {
	walkable := map[[2]int]bool{}
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			walkable[[2]int{x, y}] = true
		}
	}
	path := CoverageRoute(walkable, [2]int{0, 0}, 1)
	if len(path)-1 > 13 {
		t.Fatalf("coverage too long: %d passos (want <= 13)", len(path)-1)
	}
}

func TestCoverageRouteStartNotWalkable(t *testing.T) {
	walkable := map[[2]int]bool{{0, 0}: true}
	if path := CoverageRoute(walkable, [2]int{5, 5}, 1); path != nil {
		t.Fatalf("expected nil for non-walkable start, got %v", path)
	}
}

// TestCoverageRouteSpacing garante que, com espaçamento, a rota passa por
// poucos pontos e que todo tile fica a até spacing (Chebyshev) de algum ponto.
func TestCoverageRouteSpacing(t *testing.T) {
	walkable := map[[2]int]bool{}
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			walkable[[2]int{x, y}] = true
		}
	}
	const spacing = 10
	path := CoverageRoute(walkable, [2]int{0, 0}, spacing)
	if len(path) == 0 {
		t.Fatal("empty path")
	}
	if path[0] != [2]int{0, 0} || path[len(path)-1] != [2]int{0, 0} {
		t.Fatalf("path must start/end at (0,0): %v..%v", path[0], path[len(path)-1])
	}
	if len(path) >= 100 {
		t.Fatalf("path too long for spacing: %d", len(path))
	}
	for tile := range walkable {
		ok := false
		for _, p := range path {
			dx := tile[0] - p[0]
			if dx < 0 {
				dx = -dx
			}
			dy := tile[1] - p[1]
			if dy < 0 {
				dy = -dy
			}
			if dx <= spacing && dy <= spacing {
				ok = true
				break
			}
		}
		if !ok {
			t.Fatalf("tile %v not covered", tile)
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
