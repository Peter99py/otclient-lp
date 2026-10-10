package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"megabrain/internal/caves"
	"megabrain/internal/chunks"
	"megabrain/internal/types"
)

func (s *Server) handleChunk(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	z, err1 := strconv.Atoi(q.Get("z"))
	level, err2 := strconv.Atoi(q.Get("level"))
	cx, err3 := strconv.Atoi(q.Get("cx"))
	cy, err4 := strconv.Atoi(q.Get("cy"))
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		http.Error(w, "invalid params", http.StatusBadRequest)
		return
	}
	caveID := 0
	if v := q.Get("cave"); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil {
			http.Error(w, "invalid cave", http.StatusBadRequest)
			return
		}
		caveID = id
	}

	base := chunkKey(z, level, cx, cy)
	s.cacheMu.Lock()
	if c, ok := s.chunkCache.get(base, caveID); ok {
		s.cacheMu.Unlock()
		writeJSON(w, c)
		return
	}
	s.cacheMu.Unlock()

	var chunk *chunks.Chunk
	s.mapper.WithMaps(z, func(static, cm map[[2]int]int) {
		chunk = chunks.Get(static, cm, z, level, cx, cy, caveID)
	})
	if chunk == nil {
		http.Error(w, "invalid level", http.StatusBadRequest)
		return
	}

	s.cacheMu.Lock()
	s.chunkCache.put(base, caveID, chunk)
	s.cacheMu.Unlock()

	writeJSON(w, chunk)
}

// handleKnown devolve os tiles já mapeados num retângulo (x, y, w, h) do andar z.
func (s *Server) handleKnown(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	z, e1 := strconv.Atoi(q.Get("z"))
	x, e2 := strconv.Atoi(q.Get("x"))
	y, e3 := strconv.Atoi(q.Get("y"))
	width, e4 := strconv.Atoi(q.Get("w"))
	height, e5 := strconv.Atoi(q.Get("h"))
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil {
		http.Error(w, "invalid params", http.StatusBadRequest)
		return
	}
	if width <= 0 || height <= 0 || width > 512 || height > 512 {
		http.Error(w, "invalid size", http.StatusBadRequest)
		return
	}
	tiles := s.mapper.KnownRect(z, x, y, x+width-1, y+height-1)
	if tiles == nil {
		tiles = []types.Tile{}
	}
	writeJSON(w, map[string]any{"z": z, "tiles": tiles})
}

func (s *Server) handleCaves(w http.ResponseWriter, r *http.Request) {
	// Andares que ainda não têm o cache de caves válido.
	var pending []int
	s.cacheMu.Lock()
	for _, z := range s.mapper.ZLevels() {
		if !s.caveValidZ[z] {
			pending = append(pending, z)
		}
	}
	if len(pending) == 0 {
		cs := s.caveCache
		s.cacheMu.Unlock()
		if cs == nil {
			cs = []caves.Cave{}
		}
		writeJSON(w, cs)
		return
	}
	s.cacheMu.Unlock()

	cs, err := caves.ReconcileZ(s.store, s.mapper, pending)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pendingSet := make(map[int]bool, len(pending))
	for _, z := range pending {
		pendingSet[z] = true
	}

	s.cacheMu.Lock()
	var kept, old []caves.Cave
	for _, c := range s.caveCache {
		if !pendingSet[c.Z] {
			kept = append(kept, c)
		} else {
			old = append(old, c)
		}
	}
	s.caveCache = append(kept, cs...)
	for _, z := range pending {
		s.caveValidZ[z] = true
	}
	out := s.caveCache
	changed := !cavesEqual(old, cs)
	s.cacheMu.Unlock()

	if changed {
		s.bumpCaves()
	}
	if out == nil {
		out = []caves.Cave{}
	}
	writeJSON(w, out)
}

// handleRenameCave renomeia uma cave persistida.
// Corpo: {"id": "<z>:<dbid>", "name": "<novo nome>"}.
func (s *Server) handleRenameCave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	parts := strings.SplitN(body.ID, ":", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	dbid, err := strconv.Atoi(parts[1])
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := s.store.UpdateCaveName(dbid, body.Name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Atualiza o nome no cache em memória sem forçar um Reconcile.
	s.cacheMu.Lock()
	for i := range s.caveCache {
		if s.caveCache[i].ID == body.ID {
			s.caveCache[i].Name = body.Name
		}
	}
	s.cacheMu.Unlock()

	writeJSON(w, map[string]bool{"ok": true})
}

// handleDeleteCave apaga uma cave: seus tiles (cache + banco), a cave e as rotas.
// Corpo: {"id": "<z>:<dbid>"}.
func (s *Server) handleDeleteCave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	z, dbid, ok := parseCaveID(body.ID)
	if !ok {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	s.mapper.DeleteCave(z, dbid)
	if err := s.store.DeleteTilesByCave(z, dbid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.store.DeleteCave(dbid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.cacheMu.Lock()
	delete(s.caveValidZ, z)
	s.chunkCache.clear()
	s.cacheMu.Unlock()

	writeJSON(w, map[string]bool{"ok": true})
}
