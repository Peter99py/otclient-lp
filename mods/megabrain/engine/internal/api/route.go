package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"megabrain/internal/config"
	"megabrain/internal/explorer"
	"megabrain/internal/pathfinder"
	"megabrain/internal/routes"
	"megabrain/internal/store"
)

// routeView é a representação de uma rota para a UI e para o client.
type routeView struct {
	ID         int      `json:"id"`
	Cave       int      `json:"cave"`
	Name       string   `json:"name"`
	Start      [2]int   `json:"start"`
	Waypoints  [][2]int `json:"waypoints"`
	Path       [][2]int `json:"path"`
	Directions []int    `json:"directions"`
	Length     int      `json:"length"`
	UpdatedAt  int64    `json:"updated_at"`
}

// routeSummary é a versão leve usada na listagem.
type routeSummary struct {
	ID        int    `json:"id"`
	Cave      int    `json:"cave"`
	Name      string `json:"name"`
	Length    int    `json:"length"`
	UpdatedAt int64  `json:"updated_at"`
}

func pathToDirections(path [][2]int) []int {
	if len(path) < 2 {
		return []int{}
	}
	out := make([]int, 0, len(path)-1)
	for i := 1; i < len(path); i++ {
		if d, ok := directions[[2]int{path[i][0] - path[i-1][0], path[i][1] - path[i-1][1]}]; ok {
			out = append(out, d)
		}
	}
	return out
}

func (s *Server) routeView(r store.RouteRow) routeView {
	path := routes.Decode(r.StartX, r.StartY, r.Dirs)
	return routeView{
		ID:         r.ID,
		Cave:       r.CaveID,
		Name:       r.Name,
		Start:      [2]int{r.StartX, r.StartY},
		Waypoints:  routes.Waypoints(path),
		Path:       path,
		Directions: pathToDirections(path),
		Length:     len(path),
		UpdatedAt:  r.UpdatedAt,
	}
}

// parseCaveID separa "z:dbid".
func parseCaveID(v string) (z, dbid int, ok bool) {
	parts := strings.SplitN(v, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	z, err1 := strconv.Atoi(parts[0])
	dbid, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return z, dbid, true
}

// caveComponent retorna os tiles andáveis da cave (dbid) no andar z e os limites.
// Tiles de troca de andar (escada/buraco) são excluídos: a rota não sai da cave.
func (s *Server) caveComponent(z, caveID int) (map[[2]int]bool, int, int, int, int) {
	walkable := map[[2]int]bool{}
	minX, minY, maxX, maxY := 0, 0, 0, 0
	first := true
	s.mapper.WithRouteMaps(z, func(static, caves map[[2]int]int, floor map[[2]int]bool) {
		for p, w := range static {
			if w != 1 || caves[p] != caveID || floor[p] {
				continue
			}
			walkable[p] = true
			if first {
				minX, minY, maxX, maxY = p[0], p[1], p[0], p[1]
				first = false
				continue
			}
			if p[0] < minX {
				minX = p[0]
			}
			if p[0] > maxX {
				maxX = p[0]
			}
			if p[1] < minY {
				minY = p[1]
			}
			if p[1] > maxY {
				maxY = p[1]
			}
		}
	})
	return walkable, minX, minY, maxX, maxY
}

// routeEffective devolve uma cópia do mapa estático com os tiles de troca de
// andar (escada/buraco) marcados como bloqueados, para o A* não sair da cave.
func routeEffective(static map[[2]int]int, floor map[[2]int]bool) map[[2]int]int {
	out := make(map[[2]int]int, len(static))
	for p, w := range static {
		if w == 1 && floor[p] {
			out[p] = 0
			continue
		}
		out[p] = w
	}
	return out
}

// routeSpacing devolve o espaçamento (em tiles) configurado pelo client.
func (s *Server) routeSpacing() int {
	s.mu.RLock()
	v := s.latest.RouteSpacing
	s.mu.RUnlock()
	if v < 1 {
		return config.DefaultRouteSpacing
	}
	return v
}

// pickStart escolhe o tile inicial da cobertura: a posição do jogador se estiver
// na cave, senão o tile andável de menor (x, y).
func (s *Server) pickStart(walkable map[[2]int]bool, z int) [2]int {
	s.mu.RLock()
	p := s.latest.Player
	s.mu.RUnlock()
	if p != nil && p.Z == z && walkable[[2]int{p.X, p.Y}] {
		return [2]int{p.X, p.Y}
	}
	var best [2]int
	first := true
	for t := range walkable {
		if first || t[0] < best[0] || (t[0] == best[0] && t[1] < best[1]) {
			best = t
			first = false
		}
	}
	return best
}

// buildRoutePath reconstrói a caminhada densa ligando os waypoints em sequência
// (A* entre consecutivos) e fechando o loop de volta ao primeiro.
func (s *Server) buildRoutePath(wps [][2]int, z, minX, minY, maxX, maxY int) ([][2]int, bool) {
	if len(wps) == 0 {
		return nil, false
	}
	var path [][2]int
	ok := true
	s.mapper.WithRouteMaps(z, func(static, _ map[[2]int]int, floor map[[2]int]bool) {
		if len(wps) == 1 {
			path = [][2]int{wps[0]}
			return
		}
		effective := routeEffective(static, floor)
		for i := 0; i < len(wps); i++ {
			a := wps[i]
			b := wps[(i+1)%len(wps)]
			seg := pathfinder.FindPathBounds(a, b, effective, minX-1, minY-1, maxX+1, maxY+1)
			if seg == nil {
				ok = false
				return
			}
			if i == 0 {
				path = append(path, seg...)
			} else {
				path = append(path, seg[1:]...)
			}
		}
	})
	if !ok {
		return nil, false
	}
	return path, true
}

// normalizeWaypoints remove a repetição do início no fim (loop fechado).
func normalizeWaypoints(wps [][2]int) [][2]int {
	if len(wps) > 1 && wps[0] == wps[len(wps)-1] {
		return wps[:len(wps)-1]
	}
	return wps
}

func (s *Server) handleRoutesList(w http.ResponseWriter, r *http.Request) {
	caveID, err := strconv.Atoi(r.URL.Query().Get("cave"))
	if err != nil {
		http.Error(w, "invalid cave", http.StatusBadRequest)
		return
	}
	rows, err := s.store.LoadRoutes(caveID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]routeSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, routeSummary{
			ID: r.ID, Cave: r.CaveID, Name: r.Name,
			Length: len(r.Dirs) + 1, UpdatedAt: r.UpdatedAt,
		})
	}
	writeJSON(w, out)
}

func (s *Server) handleRouteCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Cave string `json:"cave"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	z, dbid, ok := parseCaveID(body.Cave)
	if !ok {
		http.Error(w, "invalid cave", http.StatusBadRequest)
		return
	}
	cave, err := s.store.GetCave(dbid)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if cave == nil {
		http.Error(w, "cave not found", http.StatusNotFound)
		return
	}
	if cave.Explored == 0 {
		http.Error(w, "cave not fully explored", http.StatusConflict)
		return
	}
	walkable, _, _, _, _ := s.caveComponent(z, dbid)
	if len(walkable) == 0 {
		http.Error(w, "empty cave", http.StatusConflict)
		return
	}
	start := s.pickStart(walkable, z)
	path := explorer.CoverageRoute(walkable, start, s.routeSpacing())
	if len(path) == 0 {
		http.Error(w, "coverage failed", http.StatusInternalServerError)
		return
	}
	sx, sy, dirs := routes.Encode(path)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "Rota"
	}
	row := store.RouteRow{
		CaveID: dbid, Name: name, StartX: sx, StartY: sy, Dirs: dirs,
		UpdatedAt: time.Now().Unix(),
	}
	id, err := s.store.InsertRoute(row)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	row.ID = id
	s.bumpRoutes()
	writeJSON(w, s.routeView(row))
}

func (s *Server) handleRouteUpdate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID        int      `json:"id"`
		Name      string   `json:"name"`
		Waypoints [][2]int `json:"waypoints"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	row, err := s.store.GetRoute(body.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if row == nil {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}
	cave, err := s.store.GetCave(row.CaveID)
	if err != nil || cave == nil {
		http.Error(w, "cave not found", http.StatusNotFound)
		return
	}
	wps := normalizeWaypoints(body.Waypoints)
	if len(wps) == 0 {
		http.Error(w, "empty waypoints", http.StatusBadRequest)
		return
	}

	// Valida que cada waypoint é andável e não é troca de andar.
	bad := -1
	s.mapper.WithRouteMaps(cave.Z, func(static, _ map[[2]int]int, floor map[[2]int]bool) {
		for i, wp := range wps {
			if static[wp] != 1 || floor[wp] {
				bad = i
				return
			}
		}
	})
	if bad >= 0 {
		writeJSON(w, map[string]any{"ok": false, "error": "waypoint not walkable", "index": bad})
		return
	}

	path, ok := s.buildRoutePath(wps, cave.Z, cave.MinX, cave.MinY, cave.MaxX, cave.MaxY)
	if !ok {
		writeJSON(w, map[string]any{"ok": false, "error": "segment not connected"})
		return
	}
	sx, sy, dirs := routes.Encode(path)
	row.StartX, row.StartY, row.Dirs = sx, sy, dirs
	if strings.TrimSpace(body.Name) != "" {
		row.Name = strings.TrimSpace(body.Name)
	}
	row.UpdatedAt = time.Now().Unix()
	if err := s.store.UpdateRoute(*row); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.bumpRoutes()
	writeJSON(w, s.routeView(*row))
}

// handleRouteRename renomeia uma rota. Corpo: {"id": <int>, "name": "<novo>"}.
func (s *Server) handleRouteRename(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		http.Error(w, "empty name", http.StatusBadRequest)
		return
	}
	row, err := s.store.GetRoute(body.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if row == nil {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}
	if err := s.store.UpdateRouteName(body.ID, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	row.Name = name
	s.bumpRoutes()
	writeJSON(w, s.routeView(*row))
}

func (s *Server) handleRouteDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteRoute(body.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.routeMu.Lock()
	if s.activeRouteID == body.ID {
		s.activeRouteID = 0
	}
	s.routeMu.Unlock()
	s.bumpRoutes()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleRouteRecalc(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	row, err := s.store.GetRoute(body.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if row == nil {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}
	cave, err := s.store.GetCave(row.CaveID)
	if err != nil || cave == nil {
		http.Error(w, "cave not found", http.StatusNotFound)
		return
	}
	walkable, _, _, _, _ := s.caveComponent(cave.Z, row.CaveID)
	if len(walkable) == 0 {
		http.Error(w, "empty cave", http.StatusConflict)
		return
	}
	start := s.pickStart(walkable, cave.Z)
	path := explorer.CoverageRoute(walkable, start, s.routeSpacing())
	sx, sy, dirs := routes.Encode(path)
	row.StartX, row.StartY, row.Dirs = sx, sy, dirs
	row.UpdatedAt = time.Now().Unix()
	if err := s.store.UpdateRoute(*row); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.bumpRoutes()
	writeJSON(w, s.routeView(*row))
}

func (s *Server) handleRouteGet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	row, err := s.store.GetRoute(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if row == nil {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}
	writeJSON(w, s.routeView(*row))
}

func (s *Server) handleTile(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	z, e1 := strconv.Atoi(q.Get("z"))
	x, e2 := strconv.Atoi(q.Get("x"))
	y, e3 := strconv.Atoi(q.Get("y"))
	if e1 != nil || e2 != nil || e3 != nil {
		http.Error(w, "invalid params", http.StatusBadRequest)
		return
	}
	walkable := false
	caveID := 0
	s.mapper.WithMaps(z, func(static, caves map[[2]int]int) {
		walkable = static[[2]int{x, y}] == 1
		caveID = caves[[2]int{x, y}]
	})
	writeJSON(w, map[string]any{"z": z, "x": x, "y": y, "walkable": walkable, "cave_id": caveID})
}

func (s *Server) handleRouteActiveGet(w http.ResponseWriter, r *http.Request) {
	s.routeMu.Lock()
	id := s.activeRouteID
	s.routeMu.Unlock()
	active := id != 0
	var updated int64
	if active {
		if row, err := s.store.GetRoute(id); err == nil && row != nil {
			updated = row.UpdatedAt
		} else {
			// Rota não existe mais: limpa o estado stale.
			s.routeMu.Lock()
			if s.activeRouteID == id {
				s.activeRouteID = 0
			}
			s.routeMu.Unlock()
			active = false
			id = 0
		}
	}
	writeJSON(w, map[string]any{"active": active, "id": id, "updated_at": updated})
}

func (s *Server) handleRouteActiveSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	row, err := s.store.GetRoute(body.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if row == nil {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}
	s.routeMu.Lock()
	s.activeRouteID = body.ID
	s.routeMu.Unlock()
	s.bumpRoutes()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleRouteActiveClear(w http.ResponseWriter, r *http.Request) {
	s.routeMu.Lock()
	s.activeRouteID = 0
	s.routeMu.Unlock()
	s.bumpRoutes()
	writeJSON(w, map[string]bool{"ok": true})
}

// handleRouteAuto gera (se não existir) e ativa a rota da cave do jogador.
// Usado pelo client quando a exploração termina, para o personagem não parar.
func (s *Server) handleRouteAuto(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	p := s.latest.Player
	s.mu.RUnlock()
	if p == nil {
		writeJSON(w, map[string]any{"ok": false, "reason": "sem jogador"})
		return
	}
	z := p.Z

	// Garante o cave_id por tile atualizado (só o andar do jogador).
	s.reconcileZ(z)

	caveID := 0
	s.mapper.WithCaves(z, func(cm map[[2]int]int) {
		caveID = cm[[2]int{p.X, p.Y}]
	})
	if caveID == 0 {
		writeJSON(w, map[string]any{"ok": false, "reason": "cave não identificada"})
		return
	}
	cave, err := s.store.GetCave(caveID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if cave == nil {
		writeJSON(w, map[string]any{"ok": false, "reason": "cave não encontrada"})
		return
	}
	if cave.Explored == 0 {
		msg := "Auto-rota: cave não explorada — rota não gerada."
		s.addNotice(msg)
		writeJSON(w, map[string]any{"ok": false, "reason": msg})
		return
	}

	// Já existe rota? Usa a mais recente (maior updated_at).
	rows, err := s.store.LoadRoutes(caveID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	created := false
	routeID := 0
	var best store.RouteRow
	for _, r := range rows {
		if routeID == 0 || r.UpdatedAt > best.UpdatedAt {
			best = r
			routeID = r.ID
		}
	}
	if routeID == 0 {
		walkable, _, _, _, _ := s.caveComponent(z, caveID)
		if len(walkable) == 0 {
			msg := "Auto-rota: cave sem tiles andáveis — rota não gerada."
			s.addNotice(msg)
			writeJSON(w, map[string]any{"ok": false, "reason": msg})
			return
		}
		start := s.pickStart(walkable, z)
		path := explorer.CoverageRoute(walkable, start, s.routeSpacing())
		if len(path) == 0 {
			msg := "Auto-rota: falha ao gerar cobertura."
			s.addNotice(msg)
			writeJSON(w, map[string]any{"ok": false, "reason": msg})
			return
		}
		sx, sy, dirs := routes.Encode(path)
		row := store.RouteRow{
			CaveID: caveID, Name: "Rota (auto)", StartX: sx, StartY: sy, Dirs: dirs,
			UpdatedAt: time.Now().Unix(),
		}
		id, err := s.store.InsertRoute(row)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		routeID = id
		created = true
	}

	s.routeMu.Lock()
	s.activeRouteID = routeID
	s.routeMu.Unlock()
	s.bumpRoutes()

	writeJSON(w, map[string]any{"ok": true, "routeId": routeID, "created": created})
}

// activeRouteIDValue devolve o id da rota ativa (0 se nenhuma).
func (s *Server) activeRouteIDValue() int {
	s.routeMu.Lock()
	defer s.routeMu.Unlock()
	return s.activeRouteID
}

// handleRouteEntry devolve a rota já ajustada para começar no ponto mais
// próximo do jogador: um caminho de entrada até esse ponto, seguido da rota
// rotacionada (loop). `loop_start` é o índice (1-based) onde o loop começa.
func (s *Server) handleRouteEntry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	row, err := s.store.GetRoute(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if row == nil {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}
	cave, err := s.store.GetCave(row.CaveID)
	if err != nil || cave == nil {
		http.Error(w, "cave not found", http.StatusNotFound)
		return
	}

	route := routes.Decode(row.StartX, row.StartY, row.Dirs)
	if len(route) == 0 {
		writeJSON(w, map[string]any{"path": [][2]int{}, "directions": []int{}, "loop_start": 1})
		return
	}

	s.mu.RLock()
	p := s.latest.Player
	s.mu.RUnlock()

	// Sem jogador no andar da cave: devolve a rota como está.
	if p == nil || p.Z != cave.Z {
		writeJSON(w, map[string]any{
			"path": route, "directions": pathToDirections(route), "loop_start": 1,
		})
		return
	}

	player := [2]int{p.X, p.Y}
	idx := nearestIndex(route, player)

	var entry [][2]int
	if route[idx] == player {
		entry = [][2]int{player}
	} else {
		s.mapper.WithRouteMaps(cave.Z, func(static, _ map[[2]int]int, floor map[[2]int]bool) {
			entry = pathfinder.FindPathBounds(player, route[idx], routeEffective(static, floor),
				cave.MinX-1, cave.MinY-1, cave.MaxX+1, cave.MaxY+1)
		})
		if entry == nil {
			// Ponto mais próximo inalcançável: devolve a rota como está.
			writeJSON(w, map[string]any{
				"path": route, "directions": pathToDirections(route), "loop_start": 1,
			})
			return
		}
	}

	// Rotaciona a rota para começar em idx e concatena após a entrada.
	rotated := append(append([][2]int{}, route[idx:]...), route[1:idx+1]...)
	combined := append(append([][2]int{}, entry...), rotated[1:]...)

	writeJSON(w, map[string]any{
		"path":       combined,
		"directions": pathToDirections(combined),
		"loop_start": len(entry), // 1-based no client
	})
}

func nearestIndex(path [][2]int, p [2]int) int {
	best, bestD := 0, -1
	for i, t := range path {
		d := absInt(t[0]-p[0]) + absInt(t[1]-p[1])
		if bestD < 0 || d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
