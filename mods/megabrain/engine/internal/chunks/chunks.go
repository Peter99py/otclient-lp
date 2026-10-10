package chunks

import (
	"megabrain/internal/config"
)

const (
	unknown  = '0'
	walkable = '1'
	blocked  = '2'
)

var neighbors4 = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// Chunk é um bloco de ChunkSize x ChunkSize células num nível de LOD.
type Chunk struct {
	Z     int    `json:"z"`
	Level int    `json:"level"`
	CX    int    `json:"cx"`
	CY    int    `json:"cy"`
	Size  int    `json:"size"`
	Data  string `json:"data"`
}

// Get monta o chunk a partir do cache em memória do mapper (static: (x,y) ->
// walkable; caveMap: (x,y) -> cave_id). No nível L, cada célula agrega 2^L x 2^L
// tiles (andável vence). Retorna nil se o nível for inválido.
// Se caveID != 0, inclui apenas os tiles da cave (andáveis da cave + a borda
// bloqueada adjacente), para a UI mostrar só a cave selecionada.
func Get(static, caveMap map[[2]int]int, z, level, cx, cy int, caveID int) *Chunk {
	if level < 0 || level > config.MaxLOD {
		return nil
	}

	factor := 1 << level
	span := config.ChunkSize * factor
	minX := cx * span
	minY := cy * span

	cells := make([]byte, config.ChunkSize*config.ChunkSize)
	for i := range cells {
		cells[i] = unknown
	}

	for x := minX; x < minX+span; x++ {
		for y := minY; y < minY+span; y++ {
			w, ok := static[[2]int{x, y}]
			if !ok {
				continue
			}
			if caveID != 0 && !inCave(caveMap, x, y, w, caveID) {
				continue
			}
			i := (x - minX) / factor
			j := (y - minY) / factor
			idx := j*config.ChunkSize + i
			if w == 1 {
				cells[idx] = walkable
			} else if cells[idx] == unknown {
				cells[idx] = blocked
			}
		}
	}

	return &Chunk{
		Z:     z,
		Level: level,
		CX:    cx,
		CY:    cy,
		Size:  config.ChunkSize,
		Data:  string(cells),
	}
}

// inCave diz se o tile pertence à cave: andável com cave_id == caveID, ou
// bloqueado adjacente a um tile da cave (a borda).
func inCave(caveMap map[[2]int]int, x, y, w, caveID int) bool {
	if caveMap[[2]int{x, y}] == caveID {
		return true
	}
	if w == 0 {
		for _, d := range neighbors4 {
			if caveMap[[2]int{x + d[0], y + d[1]}] == caveID {
				return true
			}
		}
	}
	return false
}
