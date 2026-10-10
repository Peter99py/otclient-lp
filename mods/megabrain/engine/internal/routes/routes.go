// Package routes codifica e decodifica caminhadas densas.
//
// Uma rota é uma caminhada (lista de tiles adjacentes) que começa e termina no
// mesmo tile. Para persistir de forma compacta, guardamos o tile inicial e a
// sequência de direções (N/E/S/W); a caminhada é reconstruída por decodificação.
package routes

import "strings"

// delta mapeia o caractere de direção para o deslocamento (dx, dy).
var delta = map[byte][2]int{
	'N': {0, -1},
	'E': {1, 0},
	'S': {0, 1},
	'W': {-1, 0},
}

// DirChar converte um passo (dx, dy) no caractere de direção.
func DirChar(dx, dy int) (byte, bool) {
	switch {
	case dx == 0 && dy == -1:
		return 'N', true
	case dx == 1 && dy == 0:
		return 'E', true
	case dx == 0 && dy == 1:
		return 'S', true
	case dx == -1 && dy == 0:
		return 'W', true
	}
	return 0, false
}

// Encode converte uma caminhada em (startX, startY, dirs). Passos não
// ortogonais são ignorados (não devem ocorrer).
func Encode(path [][2]int) (int, int, string) {
	if len(path) == 0 {
		return 0, 0, ""
	}
	var b strings.Builder
	for i := 1; i < len(path); i++ {
		c, ok := DirChar(path[i][0]-path[i-1][0], path[i][1]-path[i-1][1])
		if !ok {
			continue
		}
		b.WriteByte(c)
	}
	return path[0][0], path[0][1], b.String()
}

// Decode reconstrói a caminhada a partir do tile inicial e das direções.
func Decode(x, y int, dirs string) [][2]int {
	path := make([][2]int, 0, len(dirs)+1)
	path = append(path, [2]int{x, y})
	for i := 0; i < len(dirs); i++ {
		d, ok := delta[dirs[i]]
		if !ok {
			continue
		}
		x += d[0]
		y += d[1]
		path = append(path, [2]int{x, y})
	}
	return path
}

// Handles retorna os índices dos pontos de curva da caminhada: o início, cada
// tile onde a direção muda e o fim. São os pontos editáveis na UI.
func Handles(path [][2]int) []int {
	if len(path) == 0 {
		return nil
	}
	idx := []int{0}
	for i := 1; i < len(path)-1; i++ {
		d1x, d1y := path[i][0]-path[i-1][0], path[i][1]-path[i-1][1]
		d2x, d2y := path[i+1][0]-path[i][0], path[i+1][1]-path[i][1]
		if d1x != d2x || d1y != d2y {
			idx = append(idx, i)
		}
	}
	if len(path) > 1 {
		idx = append(idx, len(path)-1)
	}
	return idx
}

// MinWaypointGap é a distância mínima (Manhattan) entre waypoints consecutivos
// derivados de uma caminhada, para evitar pontos de curva muito próximos.
const MinWaypointGap = 10

// Waypoints extrai as coordenadas dos pontos de curva, mantendo ao menos
// MinWaypointGap tiles entre pontos consecutivos. O início e o fim são sempre
// mantidos. A caminhada densa (usada na execução) não muda; isto afeta apenas a
// representação editável.
func Waypoints(path [][2]int) [][2]int {
	idx := Handles(path)
	if len(idx) == 0 {
		return nil
	}
	out := make([][2]int, 0, len(idx))
	last := path[idx[0]]
	out = append(out, last)
	for _, i := range idx[1 : len(idx)-1] {
		p := path[i]
		dx := p[0] - last[0]
		if dx < 0 {
			dx = -dx
		}
		dy := p[1] - last[1]
		if dy < 0 {
			dy = -dy
		}
		if dx+dy >= MinWaypointGap {
			out = append(out, p)
			last = p
		}
	}
	if len(idx) > 1 {
		out = append(out, path[idx[len(idx)-1]])
	}
	return out
}
