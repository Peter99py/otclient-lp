MegaBrain = {}

MegaBrain.Config = {
    radius = 10,
    serverUrl = "http://127.0.0.1:5000/tiles",
    commandUrl = "http://127.0.0.1:5000/command",
    routeUrl = "http://127.0.0.1:5000/route",
    pollMs = 1000,
    monsterStopLimit = 3,   -- para de andar com >= N monstros na tela (0 desliga)
    monsterResumeLimit = 2, -- retoma ao cair abaixo de M monstros
    resumeDelayMs = 3000,   -- espera após o último monstro antes de retomar (ms)
    autoRoute = true,       -- gera/ativa rota e muda para modo rota ao concluir a exploração
    routeSpacing = 10,      -- espaçamento (tiles) entre pontos da rota gerada
}
