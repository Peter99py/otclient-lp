package types

// Position é uma coordenada no mundo.
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
	Z int `json:"z"`
}

// Tile é um tile recebido do client.
type Tile struct {
	X           int  `json:"x"`
	Y           int  `json:"y"`
	Z           int  `json:"z"`
	Walkable    bool `json:"walkable"`    // estático (ignora criaturas)
	Dynamic     bool `json:"dynamic"`     // atual (com criaturas)
	FloorChange bool `json:"floorChange"` // escada/buraco (troca de andar)
	Reachable   bool `json:"reachable"`   // alcançável a partir do jogador (preenchido pelo backend)
}

// Creature é uma criatura visível no entorno do jogador.
type Creature struct {
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Z    int    `json:"z"`
	Kind string `json:"kind"` // monster | npc | player
}

// Payload é o corpo de POST /tiles.
type Payload struct {
	Player        *Position  `json:"player"`
	Tiles         []Tile     `json:"tiles"`
	Creatures     []Creature `json:"creatures"`
	Command       [][2]int   `json:"command"`       // caminho comandado no último tick (enviado pelo client)
	Mode          string     `json:"mode"`          // explore | route | off (enviado pelo client)
	MonsterCount  int        `json:"monsterCount"`  // monstros alcançáveis (preenchido pelo backend)
	Paused        bool       `json:"paused"`        // exploração pausada pelo gate (preenchido pelo backend)
	RouteID       int        `json:"routeId"`       // rota ativa (preenchido pelo backend)
	RouteSpacing  int        `json:"routeSpacing"`  // espaçamento da rota em tiles (config do client)
	StopLimit     int        `json:"stopLimit"`     // config do client: para com >= N monstros (0 desliga)
	ResumeLimit   int        `json:"resumeLimit"`   // config do client: retoma com < M monstros
	ResumeDelayMs int        `json:"resumeDelayMs"` // config do client: espera após o último monstro
}
