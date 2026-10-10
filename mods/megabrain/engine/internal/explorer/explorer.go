package explorer

import "megabrain/internal/config"

var neighbors = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// NearestFrontier faz BFS no mapa estático até o tile andável mais próximo que
// faz fronteira com um tile desconhecido. O tile inicial é ignorado. Tiles em
// floor (escada/buraco) não são atravessados, para não sair do andar.
func NearestFrontier(start [2]int, staticMap map[[2]int]int, floor map[[2]int]bool) ([2]int, bool) {
	if staticMap[start] != 1 {
		return [2]int{}, false
	}

	canMove := func(n [2]int) bool {
		return staticMap[n] == 1 && !floor[n]
	}

	visited := map[[2]int]bool{start: true}
	var queue [][2]int
	for _, d := range neighbors {
		n := [2]int{start[0] + d[0], start[1] + d[1]}
		if canMove(n) && !visited[n] {
			visited[n] = true
			queue = append(queue, n)
		}
	}

	for len(queue) > 0 && len(visited) <= config.MaxFrontierSearch {
		p := queue[0]
		queue = queue[1:]
		if isFrontier(p, staticMap) {
			return p, true
		}
		for _, d := range neighbors {
			n := [2]int{p[0] + d[0], p[1] + d[1]}
			if canMove(n) && !visited[n] {
				visited[n] = true
				queue = append(queue, n)
			}
		}
	}
	return [2]int{}, false
}

func isFrontier(p [2]int, staticMap map[[2]int]int) bool {
	for _, d := range neighbors {
		if _, ok := staticMap[[2]int{p[0] + d[0], p[1] + d[1]}]; !ok {
			return true
		}
	}
	return false
}

// CoverageRoute gera uma caminhada que passa por pontos representativos do
// componente conexo: um por célula de spacing×spacing (o tile andável mais
// próximo do centro da célula). Os pontos são ligados pelo caminho mais curto
// (BFS), em ordem de vizinho mais próximo, começando em start e fechando o loop.
// spacing <= 1 equivale a cobrir todos os tiles. Retorna nil se start não for
// andável.
func CoverageRoute(walkable map[[2]int]bool, start [2]int, spacing int) [][2]int {
	if !walkable[start] {
		return nil
	}
	if spacing < 1 {
		spacing = 1
	}
	reps := representatives(walkable, spacing)
	visited := map[[2]int]bool{}
	path := [][2]int{start}
	cur := start
	if reps[cur] {
		visited[cur] = true
	}

	for {
		seg := bfsPath(cur, walkable, func(p [2]int) bool { return reps[p] && !visited[p] })
		if seg == nil {
			break
		}
		for _, p := range seg[1:] {
			path = append(path, p)
			if reps[p] {
				visited[p] = true
			}
		}
		cur = seg[len(seg)-1]
		if reps[cur] {
			visited[cur] = true
		}
	}

	// Fecha o loop voltando ao início.
	if len(path) > 1 {
		if seg := bfsPath(cur, walkable, func(p [2]int) bool { return p == start }); seg != nil {
			path = append(path, seg[1:]...)
		}
	}
	return path
}

// representatives escolhe um tile andável por célula spacing×spacing: o mais
// próximo do centro da célula.
func representatives(walkable map[[2]int]bool, spacing int) map[[2]int]bool {
	type pick struct {
		best  [2]int
		bestD int
	}
	cells := map[[2]int]pick{}
	for p := range walkable {
		cx := floorDiv(p[0], spacing)
		cy := floorDiv(p[1], spacing)
		centerX := cx*spacing + spacing/2
		centerY := cy*spacing + spacing/2
		dx := p[0] - centerX
		if dx < 0 {
			dx = -dx
		}
		dy := p[1] - centerY
		if dy < 0 {
			dy = -dy
		}
		d := dx + dy
		cur, ok := cells[[2]int{cx, cy}]
		if !ok || d < cur.bestD {
			cells[[2]int{cx, cy}] = pick{best: p, bestD: d}
		}
	}
	reps := make(map[[2]int]bool, len(cells))
	for _, c := range cells {
		reps[c.best] = true
	}
	return reps
}

// floorDiv divide arredondando para baixo (correto para coordenadas negativas).
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// bfsPath retorna o caminho mais curto de from até o primeiro tile que satisfaz
// stop (BFS sobre os tiles andáveis). Retorna nil se não encontrar.
func bfsPath(from [2]int, walkable map[[2]int]bool, stop func([2]int) bool) [][2]int {
	parent := map[[2]int][2]int{from: from}
	queue := [][2]int{from}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if stop(p) {
			return reconstruct(parent, from, p)
		}
		for _, d := range neighbors {
			n := [2]int{p[0] + d[0], p[1] + d[1]}
			if walkable[n] {
				if _, ok := parent[n]; !ok {
					parent[n] = p
					queue = append(queue, n)
				}
			}
		}
	}
	return nil
}

func reconstruct(parent map[[2]int][2]int, from, to [2]int) [][2]int {
	var path [][2]int
	for q := to; ; q = parent[q] {
		path = append(path, q)
		if q == from {
			break
		}
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// ReachableSet faz BFS no mapa estático a partir de start e retorna o conjunto
// de tiles andáveis alcançáveis, limitado a max tiles visitados.
func ReachableSet(start [2]int, staticMap map[[2]int]int, max int) map[[2]int]bool {
	visited := map[[2]int]bool{start: true}
	if staticMap[start] != 1 {
		return visited
	}
	queue := [][2]int{start}
	for len(queue) > 0 && len(visited) < max {
		p := queue[0]
		queue = queue[1:]
		for _, d := range neighbors {
			n := [2]int{p[0] + d[0], p[1] + d[1]}
			if staticMap[n] == 1 && !visited[n] {
				visited[n] = true
				queue = append(queue, n)
			}
		}
	}
	return visited
}
