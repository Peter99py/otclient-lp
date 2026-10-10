package mapper

import (
	"sort"
	"sync"
	"time"

	"megabrain/internal/store"
	"megabrain/internal/types"
)

// neighbors4 são os vizinhos ortogonais, usados para decidir a borda.
var neighbors4 = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// Mapper aplica a regra monotônica e persiste apenas o diff.
// O cache é protegido por RWMutex: leituras concorrentes (RLock) e escrita
// exclusiva (Lock).
type Mapper struct {
	store *store.Store
	mu    sync.RWMutex
	cache map[int]map[[2]int]int  // z -> (x, y) -> walkable
	caves map[int]map[[2]int]int  // z -> (x, y) -> cave_id
	floor map[int]map[[2]int]bool // z -> (x, y) -> troca de andar (escada/buraco)
}

// New cria um Mapper.
func New(s *store.Store) *Mapper {
	return &Mapper{store: s}
}

// ensureCache carrega o cache uma única vez (idempotente).
func (m *Mapper) ensureCache() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cache != nil {
		return nil
	}
	all, err := m.store.LoadAll()
	if err != nil {
		return err
	}
	m.cache = make(map[int]map[[2]int]int)
	m.caves = make(map[int]map[[2]int]int)
	m.floor = make(map[int]map[[2]int]bool)
	for k, d := range all {
		z := k[2]
		zmap := m.cache[z]
		if zmap == nil {
			zmap = make(map[[2]int]int)
			m.cache[z] = zmap
		}
		zmap[[2]int{k[0], k[1]}] = d.Walkable
		if d.FloorChange != 0 {
			fmap := m.floor[z]
			if fmap == nil {
				fmap = make(map[[2]int]bool)
				m.floor[z] = fmap
			}
			fmap[[2]int{k[0], k[1]}] = true
		}
		if d.CaveID != 0 {
			cmap := m.caves[z]
			if cmap == nil {
				cmap = make(map[[2]int]int)
				m.caves[z] = cmap
			}
			cmap[[2]int{k[0], k[1]}] = d.CaveID
		}
	}
	return nil
}

// Merge grava só o diff (novo ou upgrade 0 -> 1) e retorna as linhas gravadas.
// Tiles bloqueados só são gravados se forem borda (adjacentes a um tile andável),
// evitando persistir dados desnecessários longe de qualquer área andável.
func (m *Mapper) Merge(p types.Payload) ([]store.TileRow, error) {
	if err := m.ensureCache(); err != nil {
		return nil, err
	}
	now := time.Now().Unix()

	payloadWalkable := make(map[[3]int]bool, len(p.Tiles))
	for _, t := range p.Tiles {
		if t.Walkable {
			payloadWalkable[[3]int{t.X, t.Y, t.Z}] = true
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var rows []store.TileRow

	for _, t := range p.Tiles {
		zmap := m.cache[t.Z]
		if zmap == nil {
			zmap = make(map[[2]int]int)
			m.cache[t.Z] = zmap
		}
		fmap := m.floor[t.Z]
		if fmap == nil {
			fmap = make(map[[2]int]bool)
			m.floor[t.Z] = fmap
		}
		key := [2]int{t.X, t.Y}
		walkable := 0
		if t.Walkable {
			walkable = 1
		}
		if walkable == 0 && !m.isBorder(key, t.Z, payloadWalkable) {
			continue
		}
		current, ok := zmap[key]
		curFloor := fmap[key]
		if !ok || (current == 0 && walkable == 1) || (!curFloor && t.FloorChange) {
			zmap[key] = walkable
			if t.FloorChange {
				fmap[key] = true
			}
			rows = append(rows, store.TileRow{
				X: t.X, Y: t.Y, Z: t.Z, Walkable: walkable,
				FloorChange: boolToInt(t.FloorChange), LastSeen: now,
			})
		}
	}

	if err := m.store.UpsertMany(rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// isBorder diz se um tile bloqueado é adjacente a algum tile andável
// (no payload atual ou no cache). Deve ser chamado com o lock de escrita ativo.
func (m *Mapper) isBorder(key [2]int, z int, payloadWalkable map[[3]int]bool) bool {
	zmap := m.cache[z]
	for _, d := range neighbors4 {
		n := [2]int{key[0] + d[0], key[1] + d[1]}
		if payloadWalkable[[3]int{n[0], n[1], z}] {
			return true
		}
		if zmap != nil && zmap[n] == 1 {
			return true
		}
	}
	return false
}

// WithStatic executa fn com o mapa estático do andar z sob RLock. O mapa não
// deve ser retido após fn retornar (é o cache interno, mutável).
func (m *Mapper) WithStatic(z int, fn func(static map[[2]int]int)) {
	if err := m.ensureCache(); err != nil {
		fn(nil)
		return
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	fn(m.cache[z])
}

// WithMaps executa fn com os mapas estático e de caves do andar z sob um único
// RLock. Os mapas não devem ser retidos após fn retornar.
func (m *Mapper) WithMaps(z int, fn func(static, caves map[[2]int]int)) {
	if err := m.ensureCache(); err != nil {
		fn(nil, nil)
		return
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	fn(m.cache[z], m.caves[z])
}

// WithRouteMaps executa fn com os mapas estático, de caves e de troca de andar
// do andar z sob um único RLock. Os mapas não devem ser retidos após fn retornar.
func (m *Mapper) WithRouteMaps(z int, fn func(static, caves map[[2]int]int, floor map[[2]int]bool)) {
	if err := m.ensureCache(); err != nil {
		fn(nil, nil, nil)
		return
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	fn(m.cache[z], m.caves[z], m.floor[z])
}

// WithCaves executa fn com o mapa cave_id do andar z sob RLock. O mapa não
// deve ser retido após fn retornar.
func (m *Mapper) WithCaves(z int, fn func(caves map[[2]int]int)) {
	if err := m.ensureCache(); err != nil {
		fn(nil)
		return
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	fn(m.caves[z])
}

// SetCaveIDs atualiza o cave_id em memória e persiste só o diff.
func (m *Mapper) SetCaveIDs(rows []store.CaveTile) error {
	if len(rows) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range rows {
		cmap := m.caves[r.Z]
		if cmap == nil {
			cmap = make(map[[2]int]int)
			m.caves[r.Z] = cmap
		}
		cmap[[2]int{r.X, r.Y}] = r.CaveID
	}
	return m.store.UpdateCaveIDs(rows)
}

// DeleteCave remove do cache os tiles andáveis de uma cave (e o cave_id deles).
func (m *Mapper) DeleteCave(z, caveID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cmap := m.caves[z]
	zmap := m.cache[z]
	fmap := m.floor[z]
	for p, c := range cmap {
		if c != caveID {
			continue
		}
		delete(cmap, p)
		if zmap != nil {
			delete(zmap, p)
		}
		if fmap != nil {
			delete(fmap, p)
		}
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ZLevels retorna os andares presentes no cache, em ordem crescente.
func (m *Mapper) ZLevels() []int {
	if err := m.ensureCache(); err != nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	zs := make([]int, 0, len(m.cache))
	for z := range m.cache {
		zs = append(zs, z)
	}
	sort.Ints(zs)
	return zs
}

// KnownRect retorna os tiles já mapeados (estáticos) dentro do retângulo
// [minX, maxX] x [minY, maxY] do andar z. Itera apenas a área pedida.
func (m *Mapper) KnownRect(z, minX, minY, maxX, maxY int) []types.Tile {
	if err := m.ensureCache(); err != nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	zmap := m.cache[z]
	if zmap == nil {
		return nil
	}
	var out []types.Tile
	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			w, ok := zmap[[2]int{x, y}]
			if !ok {
				continue
			}
			out = append(out, types.Tile{X: x, Y: y, Z: z, Walkable: w == 1})
		}
	}
	return out
}
