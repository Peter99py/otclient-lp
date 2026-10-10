package caves

import (
	"fmt"
	"time"

	"megabrain/internal/mapper"
	"megabrain/internal/store"
)

var neighbors = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// Bounds é o retângulo que envolve uma cave.
type Bounds struct {
	MinX int `json:"min_x"`
	MinY int `json:"min_y"`
	MaxX int `json:"max_x"`
	MaxY int `json:"max_y"`
}

// Cave é um componente conexo de tiles andáveis.
type Cave struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Z         int    `json:"z"`
	Tiles     int    `json:"tiles"`
	Bounds    Bounds `json:"bounds"`
	Explored  bool   `json:"explored"`
	UpdatedAt int64  `json:"updated_at"`
}

// geometry descreve um componente calculado.
type geometry struct {
	minX, minY, maxX, maxY int
	anchorX, anchorY       int // tile do componente (lexicograficamente menor)
	explored               bool
}

// ReconcileZ recalcula as caves apenas dos andares informados, a partir do
// cache em memória do mapper, sincroniza a tabela persistida e grava o cave_id
// de cada tile (só o diff).
func ReconcileZ(s *store.Store, m *mapper.Mapper, zs []int) ([]Cave, error) {
	var out []Cave
	for _, z := range zs {
		walkable := make(map[[2]int]bool)
		known := make(map[[2]int]bool)
		m.WithStatic(z, func(static map[[2]int]int) {
			for p, w := range static {
				known[p] = true
				if w == 1 {
					walkable[p] = true
				}
			}
		})
		cur := make(map[[2]int]int)
		m.WithCaves(z, func(caves map[[2]int]int) {
			for p, c := range caves {
				cur[p] = c
			}
		})

		cs, updates, err := reconcileZ(s, z, walkable, known, cur)
		if err != nil {
			return nil, err
		}
		if err := m.SetCaveIDs(updates); err != nil {
			return nil, err
		}
		out = append(out, cs...)
	}
	return out, nil
}

func reconcileZ(s *store.Store, z int, walkable, known map[[2]int]bool, cur map[[2]int]int) ([]Cave, []store.CaveTile, error) {
	persisted, err := s.LoadCaves(z)
	if err != nil {
		return nil, nil, err
	}

	byAnchor := make(map[[2]int]store.CaveRow, len(persisted))
	for _, r := range persisted {
		byAnchor[[2]int{r.AnchorX, r.AnchorY}] = r
	}

	visited := make(map[[2]int]bool)
	used := make(map[int]bool)
	var out []Cave
	var updates []store.CaveTile

	for start := range walkable {
		if visited[start] {
			continue
		}
		component := flood(start, walkable, visited)
		g := describe(component, known)

		// Âncoras persistidas dentro deste componente.
		var matches []store.CaveRow
		for _, p := range component {
			if r, ok := byAnchor[p]; ok {
				matches = append(matches, r)
			}
		}

		var row store.CaveRow
		if len(matches) == 0 {
			row = store.CaveRow{
				Z:         z,
				AnchorX:   g.anchorX,
				AnchorY:   g.anchorY,
				Name:      fmt.Sprintf("Cave (z=%d)", z),
				Explored:  boolToInt(g.explored),
				Tiles:     len(component),
				MinX:      g.minX,
				MinY:      g.minY,
				MaxX:      g.maxX,
				MaxY:      g.maxY,
				UpdatedAt: time.Now().Unix(),
			}
			id, err := s.InsertCave(row)
			if err != nil {
				return nil, nil, err
			}
			row.ID = id
		} else {
			// Mantém a mais antiga (menor id); as demais viram órfãs e são removidas no fim.
			row = matches[0]
			for _, m := range matches[1:] {
				if m.ID < row.ID {
					row = m
				}
			}
			changed := row.Tiles != len(component) ||
				row.MinX != g.minX || row.MinY != g.minY ||
				row.MaxX != g.maxX || row.MaxY != g.maxY ||
				row.Explored != boolToInt(g.explored)
			if changed {
				row.Tiles = len(component)
				row.MinX, row.MinY, row.MaxX, row.MaxY = g.minX, g.minY, g.maxX, g.maxY
				row.Explored = boolToInt(g.explored)
				row.UpdatedAt = time.Now().Unix()
				if err := s.UpdateCave(row); err != nil {
					return nil, nil, err
				}
			}
		}
		used[row.ID] = true

		// cave_id por tile (só o diff).
		for _, p := range component {
			if cur[p] != row.ID {
				updates = append(updates, store.CaveTile{X: p[0], Y: p[1], Z: z, CaveID: row.ID})
			}
		}

		out = append(out, Cave{
			ID:        fmt.Sprintf("%d:%d", z, row.ID),
			Name:      row.Name,
			Z:         z,
			Tiles:     len(component),
			Bounds:    Bounds{MinX: g.minX, MinY: g.minY, MaxX: g.maxX, MaxY: g.maxY},
			Explored:  g.explored,
			UpdatedAt: row.UpdatedAt,
		})
	}

	// Remove órfãs: caves persistidas cuja âncora não foi reutilizada.
	for _, r := range persisted {
		if !used[r.ID] {
			if err := s.DeleteCave(r.ID); err != nil {
				return nil, nil, err
			}
		}
	}

	return out, updates, nil
}

func flood(start [2]int, walkable, visited map[[2]int]bool) [][2]int {
	queue := [][2]int{start}
	visited[start] = true
	var component [][2]int
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		component = append(component, p)
		for _, d := range neighbors {
			n := [2]int{p[0] + d[0], p[1] + d[1]}
			if walkable[n] && !visited[n] {
				visited[n] = true
				queue = append(queue, n)
			}
		}
	}
	return component
}

func describe(component [][2]int, known map[[2]int]bool) geometry {
	g := geometry{
		minX: component[0][0], minY: component[0][1],
		maxX: component[0][0], maxY: component[0][1],
		anchorX: component[0][0], anchorY: component[0][1],
		explored: true,
	}
	for _, p := range component {
		if p[0] < g.minX {
			g.minX = p[0]
		}
		if p[0] > g.maxX {
			g.maxX = p[0]
		}
		if p[1] < g.minY {
			g.minY = p[1]
		}
		if p[1] > g.maxY {
			g.maxY = p[1]
		}
		// Âncora = tile do componente lexicograficamente menor (garante que a
		// âncora é um tile andável, senão o match por âncora falha e a cave é
		// recriada a cada Reconcile).
		if p[0] < g.anchorX || (p[0] == g.anchorX && p[1] < g.anchorY) {
			g.anchorX, g.anchorY = p[0], p[1]
		}
		if g.explored {
			for _, d := range neighbors {
				if !known[[2]int{p[0] + d[0], p[1] + d[1]}] {
					g.explored = false
					break
				}
			}
		}
	}
	return g
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
