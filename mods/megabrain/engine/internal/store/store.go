package store

import (
	"database/sql"
	"strconv"

	_ "modernc.org/sqlite"
)

// schemaVersion é a versão atual do schema (gravada na tabela meta).
const schemaVersion = 2

const schema = `
CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT
);

CREATE TABLE IF NOT EXISTS tiles (
  x INTEGER, y INTEGER, z INTEGER,
  walkable INTEGER,
  floor_change INTEGER NOT NULL DEFAULT 0,
  cave_id INTEGER NOT NULL DEFAULT 0,
  last_seen INTEGER,
  PRIMARY KEY (x, y, z)
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS caves (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  z          INTEGER NOT NULL,
  anchor_x   INTEGER NOT NULL,
  anchor_y   INTEGER NOT NULL,
  name       TEXT,
  explored   INTEGER NOT NULL DEFAULT 0,
  tiles      INTEGER NOT NULL DEFAULT 0,
  min_x      INTEGER NOT NULL DEFAULT 0,
  min_y      INTEGER NOT NULL DEFAULT 0,
  max_x      INTEGER NOT NULL DEFAULT 0,
  max_y      INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_caves_z ON caves(z);

CREATE TABLE IF NOT EXISTS routes (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  cave_id    INTEGER NOT NULL,
  name       TEXT NOT NULL,
  start_x    INTEGER NOT NULL,
  start_y    INTEGER NOT NULL,
  dirs       TEXT NOT NULL,
  updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_routes_cave ON routes(cave_id);
`

// Upsert monotônico: só grava quando melhora (0 -> 1).
const upsertSQL = `
INSERT INTO tiles (x, y, z, walkable, floor_change, last_seen)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(x, y, z) DO UPDATE SET
  walkable     = MAX(walkable, excluded.walkable),
  floor_change = MAX(floor_change, excluded.floor_change),
  last_seen    = excluded.last_seen
WHERE walkable < excluded.walkable OR floor_change < excluded.floor_change
`

// Store é o único ponto de acesso ao SQLite.
type Store struct {
	db *sql.DB
}

// TileRow é uma linha a gravar (observação).
type TileRow struct {
	X, Y, Z     int
	Walkable    int
	FloorChange int
	LastSeen    int64
}

// TileData é um tile lido (observação + cave).
type TileData struct {
	Walkable    int
	FloorChange int
	CaveID      int
}

// CaveRow é uma cave persistida.
type CaveRow struct {
	ID        int
	Z         int
	AnchorX   int
	AnchorY   int
	Name      string
	Explored  int
	Tiles     int
	MinX      int
	MinY      int
	MaxX      int
	MaxY      int
	UpdatedAt int64
}

// CaveTile associa um tile a uma cave.
type CaveTile struct {
	X, Y, Z int
	CaveID  int
}

// RouteRow é uma rota persistida (caminhada densa codificada em direções).
type RouteRow struct {
	ID        int    `json:"id"`
	CaveID    int    `json:"cave"`
	Name      string `json:"name"`
	StartX    int    `json:"start_x"`
	StartY    int    `json:"start_y"`
	Dirs      string `json:"-"`
	UpdatedAt int64  `json:"updated_at"`
}

// Open abre (ou cria) o banco, garante o schema e roda migrações.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db}
	// Renomeia regions -> caves ANTES do schema (senão o schema criaria uma
	// tabela caves vazia e o rename seria pulado).
	if err := s.migrateRename(); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.migrateCaveID(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.migrateFloorChange(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.migrateSchemaVersion(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// migrateRename renomeia a tabela antiga regions para caves.
func (s *Store) migrateRename() error {
	hasRegions, err := s.tableExists("regions")
	if err != nil {
		return err
	}
	if !hasRegions {
		return nil
	}
	hasCaves, err := s.tableExists("caves")
	if err != nil {
		return err
	}
	if hasCaves {
		return nil
	}
	if _, err := s.db.Exec("ALTER TABLE regions RENAME TO caves"); err != nil {
		return err
	}
	if _, err := s.db.Exec("DROP INDEX IF EXISTS idx_regions_z"); err != nil {
		return err
	}
	return nil
}

// migrateCaveID adiciona a coluna cave_id em bancos antigos.
func (s *Store) migrateCaveID() error {
	hasCaveID, err := s.columnExists("tiles", "cave_id")
	if err != nil {
		return err
	}
	if hasCaveID {
		return nil
	}
	_, err = s.db.Exec("ALTER TABLE tiles ADD COLUMN cave_id INTEGER NOT NULL DEFAULT 0")
	return err
}

// migrateFloorChange adiciona a coluna floor_change em bancos antigos.
func (s *Store) migrateFloorChange() error {
	has, err := s.columnExists("tiles", "floor_change")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	_, err = s.db.Exec("ALTER TABLE tiles ADD COLUMN floor_change INTEGER NOT NULL DEFAULT 0")
	return err
}

// migrateSchemaVersion grava a versão atual do schema na tabela meta.
func (s *Store) migrateSchemaVersion() error {
	_, err := s.db.Exec(
		"INSERT INTO meta(key, value) VALUES('schema_version', ?) "+
			"ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		strconv.Itoa(schemaVersion))
	return err
}

func (s *Store) tableExists(name string) (bool, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n)
	return n > 0, err
}

func (s *Store) columnExists(table, column string) (bool, error) {
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// Close fecha o banco.
func (s *Store) Close() error {
	return s.db.Close()
}

// UpsertMany grava o lote aplicando a regra monotônica.
func (s *Store) UpsertMany(rows []TileRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(upsertSQL)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, r := range rows {
		if _, err := stmt.Exec(r.X, r.Y, r.Z, r.Walkable, r.FloorChange, r.LastSeen); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// LoadAll carrega todos os tiles: (x, y, z) -> {walkable, floor_change, cave_id}.
func (s *Store) LoadAll() (map[[3]int]TileData, error) {
	rows, err := s.db.Query("SELECT x, y, z, walkable, floor_change, cave_id FROM tiles")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[[3]int]TileData)
	for rows.Next() {
		var x, y, z int
		var d TileData
		if err := rows.Scan(&x, &y, &z, &d.Walkable, &d.FloorChange, &d.CaveID); err != nil {
			return nil, err
		}
		out[[3]int{x, y, z}] = d
	}
	return out, rows.Err()
}

// UpdateCaveIDs grava o cave_id dos tiles informados (só o diff).
func (s *Store) UpdateCaveIDs(rows []CaveTile) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("UPDATE tiles SET cave_id = ? WHERE x = ? AND y = ? AND z = ?")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, r := range rows {
		if _, err := stmt.Exec(r.CaveID, r.X, r.Y, r.Z); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// LoadCaves retorna as caves persistidas de um andar.
func (s *Store) LoadCaves(z int) ([]CaveRow, error) {
	rows, err := s.db.Query(
		"SELECT id, z, anchor_x, anchor_y, name, explored, tiles, min_x, min_y, max_x, max_y, updated_at "+
			"FROM caves WHERE z = ?", z)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CaveRow
	for rows.Next() {
		var r CaveRow
		if err := rows.Scan(&r.ID, &r.Z, &r.AnchorX, &r.AnchorY, &r.Name, &r.Explored,
			&r.Tiles, &r.MinX, &r.MinY, &r.MaxX, &r.MaxY, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetCave retorna uma cave pelo id (nil se não existir).
func (s *Store) GetCave(id int) (*CaveRow, error) {
	var r CaveRow
	err := s.db.QueryRow(
		"SELECT id, z, anchor_x, anchor_y, name, explored, tiles, min_x, min_y, max_x, max_y, updated_at "+
			"FROM caves WHERE id = ?", id).
		Scan(&r.ID, &r.Z, &r.AnchorX, &r.AnchorY, &r.Name, &r.Explored,
			&r.Tiles, &r.MinX, &r.MinY, &r.MaxX, &r.MaxY, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// InsertCave insere uma cave e retorna o id gerado.
func (s *Store) InsertCave(r CaveRow) (int, error) {
	res, err := s.db.Exec(
		"INSERT INTO caves (z, anchor_x, anchor_y, name, explored, tiles, min_x, min_y, max_x, max_y, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		r.Z, r.AnchorX, r.AnchorY, r.Name, r.Explored, r.Tiles, r.MinX, r.MinY, r.MaxX, r.MaxY, r.UpdatedAt)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// UpdateCave atualiza geometria/estado, preservando o nome.
func (s *Store) UpdateCave(r CaveRow) error {
	_, err := s.db.Exec(
		"UPDATE caves SET explored = ?, tiles = ?, min_x = ?, min_y = ?, max_x = ?, max_y = ?, updated_at = ? "+
			"WHERE id = ?",
		r.Explored, r.Tiles, r.MinX, r.MinY, r.MaxX, r.MaxY, r.UpdatedAt, r.ID)
	return err
}

// UpdateCaveName renomeia uma cave.
func (s *Store) UpdateCaveName(id int, name string) error {
	_, err := s.db.Exec("UPDATE caves SET name = ? WHERE id = ?", name, id)
	return err
}

// DeleteTilesByCave remove os tiles de uma cave (por cave_id).
func (s *Store) DeleteTilesByCave(z, caveID int) error {
	_, err := s.db.Exec("DELETE FROM tiles WHERE z = ? AND cave_id = ?", z, caveID)
	return err
}

// DeleteCave remove uma cave e suas rotas (sem rotas órfãs).
func (s *Store) DeleteCave(id int) error {
	if _, err := s.db.Exec("DELETE FROM routes WHERE cave_id = ?", id); err != nil {
		return err
	}
	_, err := s.db.Exec("DELETE FROM caves WHERE id = ?", id)
	return err
}

// InsertRoute insere uma rota e retorna o id gerado.
func (s *Store) InsertRoute(r RouteRow) (int, error) {
	res, err := s.db.Exec(
		"INSERT INTO routes (cave_id, name, start_x, start_y, dirs, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		r.CaveID, r.Name, r.StartX, r.StartY, r.Dirs, r.UpdatedAt)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// LoadRoutes retorna as rotas de uma cave.
func (s *Store) LoadRoutes(caveID int) ([]RouteRow, error) {
	rows, err := s.db.Query(
		"SELECT id, cave_id, name, start_x, start_y, dirs, updated_at FROM routes WHERE cave_id = ? ORDER BY id",
		caveID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RouteRow
	for rows.Next() {
		var r RouteRow
		if err := rows.Scan(&r.ID, &r.CaveID, &r.Name, &r.StartX, &r.StartY, &r.Dirs, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRoute retorna uma rota pelo id.
func (s *Store) GetRoute(id int) (*RouteRow, error) {
	var r RouteRow
	err := s.db.QueryRow(
		"SELECT id, cave_id, name, start_x, start_y, dirs, updated_at FROM routes WHERE id = ?", id).
		Scan(&r.ID, &r.CaveID, &r.Name, &r.StartX, &r.StartY, &r.Dirs, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// UpdateRoute atualiza nome e caminhada de uma rota.
func (s *Store) UpdateRoute(r RouteRow) error {
	_, err := s.db.Exec(
		"UPDATE routes SET name = ?, start_x = ?, start_y = ?, dirs = ?, updated_at = ? WHERE id = ?",
		r.Name, r.StartX, r.StartY, r.Dirs, r.UpdatedAt, r.ID)
	return err
}

// UpdateRouteName renomeia uma rota.
func (s *Store) UpdateRouteName(id int, name string) error {
	_, err := s.db.Exec("UPDATE routes SET name = ? WHERE id = ?", name, id)
	return err
}

// DeleteRoute remove uma rota.
func (s *Store) DeleteRoute(id int) error {
	_, err := s.db.Exec("DELETE FROM routes WHERE id = ?", id)
	return err
}
