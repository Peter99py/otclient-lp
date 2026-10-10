package config

const (
	// ChunkSize é o lado (em células) de um chunk.
	ChunkSize = 64
	// MaxLOD é o nível máximo da pirâmide de LOD.
	MaxLOD = 4
	// PathMargin é a margem (em tiles) ao redor de start/goal para o A*.
	PathMargin = 50
	// MaxFrontierSearch limita os tiles visitados na busca por fronteira.
	MaxFrontierSearch = 5000
	// MaxReachable limita os tiles visitados no BFS de alcançabilidade.
	MaxReachable = 20000
	// CommandLookahead é o número máximo de passos enviados por comando de andar.
	CommandLookahead = 3
	// DefaultPort é a porta preferida do servidor.
	DefaultPort = 5000
	// PortRange é quantas portas tentar a partir da preferida.
	PortRange = 10
	// DefaultRouteSpacing é o espaçamento (em tiles) entre pontos da rota.
	DefaultRouteSpacing = 10
)
