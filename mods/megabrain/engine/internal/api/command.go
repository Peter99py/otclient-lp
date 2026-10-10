package api

import (
	"net/http"

	"megabrain/internal/config"
	"megabrain/internal/explorer"
	"megabrain/internal/pathfinder"
)

// (dx, dy) -> Otc::Direction
var directions = map[[2]int]int{
	{0, -1}:  0, // North
	{1, 0}:   1, // East
	{0, 1}:   2, // South
	{-1, 0}:  3, // West
	{1, -1}:  4, // NorthEast
	{1, 1}:   5, // SouthEast
	{-1, 1}:  6, // SouthWest
	{-1, -1}: 7, // NorthWest
}

// plan é o resultado do cálculo de exploração (fronteira + A*).
type plan struct {
	start [2]int
	goal  [2]int
	path  [][2]int
	done  bool
}

// computePlan monta o mapa efetivo (estático sobrescrito pelo dinâmico),
// encontra a fronteira mais próxima e calcula o caminho até ela.
// O segundo retorno é false quando não há jogador conhecido.
func (s *Server) computePlan() (plan, bool) {
	s.mu.RLock()
	p := s.latest
	s.mu.RUnlock()

	if p.Player == nil {
		return plan{}, false
	}

	z := p.Player.Z
	start := [2]int{p.Player.X, p.Player.Y}

	var result plan
	s.mapper.WithRouteMaps(z, func(static, _ map[[2]int]int, floor map[[2]int]bool) {
		effective := make(map[[2]int]int, len(static))
		for k, v := range static {
			effective[k] = v
		}
		for _, t := range p.Tiles {
			if t.Z == z {
				v := 0
				if t.Dynamic {
					v = 1
				}
				effective[[2]int{t.X, t.Y}] = v
			}
		}
		// Bloqueia troca de andar (escadas/buracos): não sair da cave.
		for fp := range floor {
			effective[fp] = 0
		}

		goal, ok := explorer.NearestFrontier(start, static, floor)
		if !ok {
			result = plan{start: start, done: true}
			return
		}

		path := pathfinder.FindPath(start, goal, effective)
		if path == nil {
			path = [][2]int{}
		}
		result = plan{start: start, goal: goal, path: path}
	})
	return result, true
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	pl, ok := s.computePlan()
	if !ok {
		writeJSON(w, map[string]any{"directions": []int{}})
		return
	}
	if pl.done {
		writeJSON(w, map[string]any{"directions": []int{}, "done": true})
		return
	}

	// Envia até CommandLookahead passos do caminho; se o caminho for mais curto
	// (fronteira perto ou desvio de monstro), o lote cai naturalmente para 2 ou 1.
	dirs := make([]int, 0, config.CommandLookahead)
	for i := 1; i < len(pl.path) && len(dirs) < config.CommandLookahead; i++ {
		dir, ok := directions[[2]int{pl.path[i][0] - pl.path[i-1][0], pl.path[i][1] - pl.path[i-1][1]}]
		if !ok {
			break
		}
		dirs = append(dirs, dir)
	}
	writeJSON(w, map[string]any{"directions": dirs, "goal": pl.goal, "path": pl.path})
}

// handlePath expõe o plano somente-leitura para a UI (mesma lógica do comando).
func (s *Server) handlePath(w http.ResponseWriter, r *http.Request) {
	pl, ok := s.computePlan()
	if !ok {
		writeJSON(w, map[string]any{"path": [][2]int{}, "done": false})
		return
	}
	writeJSON(w, map[string]any{"goal": pl.goal, "path": pl.path, "done": pl.done})
}
