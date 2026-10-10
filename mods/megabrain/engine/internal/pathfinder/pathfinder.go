package pathfinder

import (
	"container/heap"

	"megabrain/internal/config"
)

var neighbors = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

type point struct {
	x, y int
}

type node struct {
	p      point
	g      int
	f      int
	parent *node
}

type pq []*node

func (q pq) Len() int           { return len(q) }
func (q pq) Less(i, j int) bool { return q[i].f < q[j].f }
func (q pq) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *pq) Push(x any)        { *q = append(*q, x.(*node)) }
func (q *pq) Pop() any {
	old := *q
	n := old[len(old)-1]
	*q = old[:len(old)-1]
	return n
}

// FindPath roda A* no mapa efetivo (estático + observações dinâmicas).
// Retorna a lista de (x, y) de start até goal, ou nil se não houver caminho.
func FindPath(start, goal [2]int, effective map[[2]int]int) [][2]int {
	minX := min(start[0], goal[0]) - config.PathMargin
	maxX := max(start[0], goal[0]) + config.PathMargin
	minY := min(start[1], goal[1]) - config.PathMargin
	maxY := max(start[1], goal[1]) + config.PathMargin
	return FindPathBounds(start, goal, effective, minX, minY, maxX, maxY)
}

// FindPathBounds é como FindPath, mas com limites explícitos de busca.
func FindPathBounds(start, goal [2]int, effective map[[2]int]int, minX, minY, maxX, maxY int) [][2]int {
	startP := point{start[0], start[1]}
	goalP := point{goal[0], goal[1]}

	inBounds := func(p point) bool {
		return p.x >= minX && p.x <= maxX && p.y >= minY && p.y <= maxY
	}
	walkable := func(p point) bool {
		if p == startP {
			return true // o jogador está em cima do próprio tile
		}
		return effective[[2]int{p.x, p.y}] == 1
	}

	open := &pq{}
	heap.Init(open)
	heap.Push(open, &node{p: startP, g: 0, f: heuristic(startP, goalP)})

	best := map[point]int{startP: 0}
	var goalNode *node

	for open.Len() > 0 {
		cur := heap.Pop(open).(*node)
		if cur.g > best[cur.p] {
			continue // entrada obsoleta
		}
		if cur.p == goalP {
			goalNode = cur
			break
		}
		for _, d := range neighbors {
			np := point{cur.p.x + d[0], cur.p.y + d[1]}
			if !inBounds(np) || !walkable(np) {
				continue
			}
			ng := cur.g + 1
			if old, ok := best[np]; ok && old <= ng {
				continue
			}
			best[np] = ng
			heap.Push(open, &node{p: np, g: ng, f: ng + heuristic(np, goalP), parent: cur})
		}
	}

	if goalNode == nil {
		return nil
	}

	var path [][2]int
	for n := goalNode; n != nil; n = n.parent {
		path = append(path, [2]int{n.p.x, n.p.y})
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func heuristic(a, b point) int {
	dx := a.x - b.x
	if dx < 0 {
		dx = -dx
	}
	dy := a.y - b.y
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}
