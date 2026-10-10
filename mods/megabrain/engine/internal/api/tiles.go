package api

import (
	"encoding/json"
	"net/http"

	"megabrain/internal/config"
	"megabrain/internal/explorer"
	"megabrain/internal/types"
)

func (s *Server) handleTiles(w http.ResponseWriter, r *http.Request) {
	var p types.Payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	rows, err := s.mapper.Merge(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	zs := s.invalidate(rows)

	// Mantém o cave_id do andar do jogador atualizado sem reconciliar o mapa
	// inteiro (só quando aquele andar recebeu mudanças neste tick).
	if p.Player != nil && zs[p.Player.Z] {
		s.reconcileZ(p.Player.Z)
	}

	// Alcançabilidade a partir do jogador (mesma região andável).
	reachable := map[[2]int]bool{}
	if p.Player != nil {
		s.mapper.WithStatic(p.Player.Z, func(static map[[2]int]int) {
			reachable = explorer.ReachableSet(
				[2]int{p.Player.X, p.Player.Y}, static, config.MaxReachable)
		})
	}

	// Anota os tiles do entorno e conta apenas monstros alcançáveis.
	for i := range p.Tiles {
		p.Tiles[i].Reachable = reachable[[2]int{p.Tiles[i].X, p.Tiles[i].Y}]
	}
	count := 0
	for _, c := range p.Creatures {
		if c.Kind == "monster" && reachable[[2]int{c.X, c.Y}] {
			count++
		}
	}

	paused := s.applyGate(count, p.StopLimit, p.ResumeLimit, p.ResumeDelayMs)
	p.MonsterCount = count
	p.Paused = paused
	p.RouteID = s.activeRouteIDValue()

	s.mu.Lock()
	s.latest = p
	s.mu.Unlock()

	writeJSON(w, map[string]any{
		"ok": true, "paused": paused, "monsterCount": count,
		"routeId": s.activeRouteIDValue(),
	})
}

func (s *Server) handleData(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	p := s.latest
	s.mu.RUnlock()
	writeJSON(w, p)
}
