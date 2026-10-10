package api

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"megabrain/internal/caves"
	"megabrain/internal/config"
	"megabrain/internal/mapper"
	"megabrain/internal/store"
	"megabrain/internal/types"
	"megabrain/internal/web"
)

// maxChunkCache limita o número de chunks em cache no servidor.
const maxChunkCache = 4096

// Server guarda o estado compartilhado entre os handlers.
type Server struct {
	store  *store.Store
	mapper *mapper.Mapper

	mu     sync.RWMutex
	latest types.Payload

	cacheMu    sync.Mutex
	chunkCache *chunkCache
	caveCache  []caves.Cave
	caveValidZ map[int]bool // andares com caveCache válido

	gateMu       sync.Mutex
	gatePaused   bool
	gateResumeAt int64

	routeMu       sync.Mutex
	activeRouteID int

	noticesMu sync.Mutex
	notices   []Notice
	noticeSeq int

	revMu     sync.Mutex
	revTiles  int64
	revCaves  int64
	revRoutes int64
}

// New cria o servidor.
func New(s *store.Store, m *mapper.Mapper) *Server {
	return &Server{
		store:      s,
		mapper:     m,
		chunkCache: newChunkCache(maxChunkCache),
		caveValidZ: make(map[int]bool),
	}
}

// Router monta as rotas HTTP.
func (s *Server) Router() http.Handler {
	staticFS, err := fs.Sub(web.Files, "static")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /tiles", s.handleTiles)
	mux.HandleFunc("GET /data", s.handleData)
	mux.HandleFunc("GET /chunk", s.handleChunk)
	mux.HandleFunc("GET /known", s.handleKnown)
	mux.HandleFunc("GET /caves", s.handleCaves)
	mux.HandleFunc("POST /caves/rename", s.handleRenameCave)
	mux.HandleFunc("POST /caves/delete", s.handleDeleteCave)
	mux.HandleFunc("GET /command", s.handleCommand)
	mux.HandleFunc("GET /path", s.handlePath)
	mux.HandleFunc("GET /routes", s.handleRoutesList)
	mux.HandleFunc("POST /routes", s.handleRouteCreate)
	mux.HandleFunc("POST /routes/update", s.handleRouteUpdate)
	mux.HandleFunc("POST /routes/rename", s.handleRouteRename)
	mux.HandleFunc("POST /routes/delete", s.handleRouteDelete)
	mux.HandleFunc("POST /routes/recalc", s.handleRouteRecalc)
	mux.HandleFunc("GET /route", s.handleRouteGet)
	mux.HandleFunc("GET /route/entry", s.handleRouteEntry)
	mux.HandleFunc("GET /tile", s.handleTile)
	mux.HandleFunc("GET /route/active", s.handleRouteActiveGet)
	mux.HandleFunc("POST /route/active", s.handleRouteActiveSet)
	mux.HandleFunc("POST /route/active/clear", s.handleRouteActiveClear)
	mux.HandleFunc("POST /route/auto", s.handleRouteAuto)
	mux.HandleFunc("GET /notices", s.handleNotices)
	mux.HandleFunc("GET /rev", s.handleRev)
	mux.HandleFunc("GET /", s.handleIndex)
	mux.Handle("GET /ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(staticFS))))
	return gzipMiddleware(mux)
}

// gzipMiddleware comprime as respostas JSON quando o cliente aceita gzip.
// Os assets estáticos (/ui/) ficam de fora para preservar range requests.
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") ||
			strings.HasPrefix(r.URL.Path, "/ui/") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, w: gz}, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	w io.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	return g.w.Write(b)
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	g.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(code)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func chunkKey(z, level, cx, cy int) string {
	return fmt.Sprintf("%d:%d:%d:%d", z, level, cx, cy)
}

// floorDiv divide arredondando para baixo (correto para coordenadas negativas).
func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// invalidate descarta os chunks afetados pelas linhas gravadas, marca os
// andares afetados como desatualizados e devolve o conjunto desses andares.
// Sem linhas gravadas, nada muda.
func (s *Server) invalidate(rows []store.TileRow) map[int]bool {
	zs := make(map[int]bool)
	if len(rows) == 0 {
		return zs
	}
	s.cacheMu.Lock()
	seen := make(map[string]bool)
	for _, r := range rows {
		zs[r.Z] = true
		for level := 0; level <= config.MaxLOD; level++ {
			span := config.ChunkSize << level
			base := chunkKey(r.Z, level, floorDiv(r.X, span), floorDiv(r.Y, span))
			if seen[base] {
				continue
			}
			seen[base] = true
			s.chunkCache.remove(base)
		}
	}
	for z := range zs {
		delete(s.caveValidZ, z)
	}
	s.cacheMu.Unlock()
	s.bumpTiles()
	return zs
}

// reconcileZ recalcula as caves de um andar e atualiza o cache de caves.
// Só incrementa a revisão de caves se a lista daquele andar realmente mudou.
func (s *Server) reconcileZ(z int) {
	cs, err := caves.ReconcileZ(s.store, s.mapper, []int{z})
	if err != nil {
		return
	}
	s.cacheMu.Lock()
	var kept, old []caves.Cave
	for _, c := range s.caveCache {
		if c.Z != z {
			kept = append(kept, c)
		} else {
			old = append(old, c)
		}
	}
	s.caveCache = append(kept, cs...)
	s.caveValidZ[z] = true
	changed := !cavesEqual(old, cs)
	s.cacheMu.Unlock()
	if changed {
		s.bumpCaves()
	}
}

// cavesEqual compara duas listas de caves (ordem irrelevante).
func cavesEqual(a, b []caves.Cave) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]caves.Cave, len(a))
	for _, c := range a {
		m[c.ID] = c
	}
	for _, c := range b {
		o, ok := m[c.ID]
		if !ok || o.Name != c.Name || o.Tiles != c.Tiles || o.Explored != c.Explored || o.Bounds != c.Bounds {
			return false
		}
	}
	return true
}

func (s *Server) bumpTiles() {
	s.revMu.Lock()
	s.revTiles++
	s.revMu.Unlock()
}

func (s *Server) bumpCaves() {
	s.revMu.Lock()
	s.revCaves++
	s.revMu.Unlock()
}

func (s *Server) bumpRoutes() {
	s.revMu.Lock()
	s.revRoutes++
	s.revMu.Unlock()
}

// handleRev devolve os contadores de revisão (tiles/caves/routes) para o
// frontend invalidar seletivamente o que mudou.
func (s *Server) handleRev(w http.ResponseWriter, r *http.Request) {
	s.revMu.Lock()
	out := map[string]int64{"tiles": s.revTiles, "caves": s.revCaves, "routes": s.revRoutes}
	s.revMu.Unlock()
	writeJSON(w, out)
}

// applyGate aplica a histerese (para em >= stop, retoma em < resume) com delay
// de retomada e retorna o estado pausado. stop <= 0 desliga o gate.
func (s *Server) applyGate(count, stop, resume, delayMs int) bool {
	s.gateMu.Lock()
	defer s.gateMu.Unlock()
	now := time.Now().UnixMilli()

	if stop <= 0 {
		s.gatePaused = false
		s.gateResumeAt = 0
	} else if count >= stop {
		s.gateResumeAt = 0
		s.gatePaused = true
	} else if s.gatePaused {
		if count < resume {
			if s.gateResumeAt == 0 {
				s.gateResumeAt = now + int64(delayMs)
			} else if now >= s.gateResumeAt {
				s.gatePaused = false
				s.gateResumeAt = 0
			}
		} else {
			s.gateResumeAt = 0
		}
	}
	return s.gatePaused
}
