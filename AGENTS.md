# AGENTS.md

## 1. Economia de tokens (obrigatório)
- Leia só o necessário (`grep`/`find_path`, intervalos de linhas). Nunca releia o que escreveu.
- Resposta = o que foi feito, arquivos tocados, validação, riscos. Sem repetir código/pedido.
- Diff mínimo: sem reformatação, renome ou correção não pedida. Não gere docs novas por padrão.

## 2. Modos de operação
**Passivo (padrão).** Ler/analisar/explicar. Não edita, não gera código, não altera estado. Se o pedido implicar mudança, confirme o escopo antes.
**Ativo.** Só quando o usuário pedir explicitamente. Atue só no escopo pedido; não commite, não crie branch, não rode migration/comando destrutivo. Ao terminar: informe arquivos tocados, validação e riscos.

## 3. Autoverificação
- Após executar código (modo ativo): releia o próprio diff e remova código morto/duplicado que você introduziu (variáveis/funções/imports sem uso, branches inalcançáveis, blocos repetidos). Não deixe integração pela metade nem código comentado.
- Rode só o build/teste relevante. Se não rodou, diga por quê. Nunca declare validação que não aconteceu.
- **Testes:** só persista testes com valor a longo prazo (proteção contra regressão de lógica central). Testes descartáveis/de depuração não ficam no repositório.

## 4. MegaBrain (módulo) — organização
Escopo: `mods/megabrain/` (módulo Lua do client + backend Go).

### Dois mapas (regra de ouro)
| Mapa | Fonte | Persiste? | Usado para |
|---|---|---|---|
| Estático | `tile:isWalkable(true)` | Sim (SQLite) | exploração/cobertura, caves |
| Dinâmico | `tile:isWalkable(false)` | Não (memória) | A* e comando de andar |
- Estático é monotônico (andável nunca regride) → elimina ruído de monstros/magias.
- Comando de andar e A* **sempre** usam o dinâmico. O client envia os dois flags por tile.
- **Escada/buraco** (`floorChange`): o client marca pela cor do minimapa (210–213). Persistido como o estático. O Explorar e a rota **nunca** pisam nesses tiles (não sair do andar).

### Lua — módulo do client
`megabrain.otmod` (`sandboxed: true` → scripts compartilham o namespace global `MegaBrain`; `@onLoad: init()` / `@onUnload: terminate()`). Scripts em `scripts:` (ordem importa), não `dofile`. Loop: `cycleEvent`; persistência: `g_settings`.
```
mods/megabrain/
├── megabrain.otmod / .lua / .otui   # módulo, init/terminate, MainWindow (Ativação/Config)
├── Makefile                          # build/perms/run/vet/test/check-lua
├── core/                             # config, logger, persist, collector, sender, navigator, backend
└── engine/                           # backend Go
```

### Go (`engine/internal/`) — backend
Binário único, sem runtime. `api` (rotas) → `mapper`/`caves`/`chunks`/`explorer`/`pathfinder`/`routes` (serviços) → `store` (SQL).
```
engine/
├── main.go            # data-dir, porta+fallback (-port), arquivo de porta
└── internal/
    ├── config/ types/ store/ mapper/ caves/ chunks/
    ├── explorer/      # frontier + cobertura (CoverageRoute)
    ├── pathfinder/    # A* (FindPath / FindPathBounds)
    ├── routes/        # encode/decode de caminhada (dirs) + handles
    ├── api/           # handlers HTTP (route.go = rotas)
    └── web/           # UI embutida (static/; route.js = editor)
```
- Binário roda **no lugar** (`/mods/megabrain/engine/`); DB e `port` em `<writeDir>/megabrain/`.
- Build: `make build`; cross: `engine/build/build.sh`.

### Persistência (SQLite)
- `tiles(x,y,z,walkable,floor_change,cave_id,last_seen)` PK `(x,y,z)` `WITHOUT ROWID`; upsert monotônico (só `0→1`), persiste só o diff; cave mapeada → zero writes.
- `caves(...)` + `routes(id,cave_id,name,start_x,start_y,dirs,updated_at)`.
- Âncora da cave = **tile do componente** (lexicograficamente menor), não `(minX,minY)` — senão o match por âncora falha e a cave é recriada a cada `Reconcile` (churn de IDs).
- Apagar cave (`POST /caves/delete`) remove tiles (cache + banco), a cave e as rotas dela.
- Notificações recentes ficam **só em memória** (`api/notices.go`, `GET /notices`), não vão para o banco.

### Rotas (patrulha)
- Rota = caminhada densa (cobertura da cave) que começa e termina no mesmo tile; `dirs` = string `N/E/S/W`.
- Geração: `explorer.CoverageRoute` passa por **pontos representativos** (1 por célula de `spacing`×`spacing`, o tile andável mais próximo do centro), ligados pelo caminho mais curto (BFS) em ordem de vizinho mais próximo, fechando o loop. `spacing` vem do client (`routeSpacing`, default 10); `spacing=1` = cobrir todos os tiles.
- Edição: handles (pontos de curva) são editáveis; a caminhada é reconstruída por A* entre handles (`FindPathBounds`) e o loop é fechado. `routes.Waypoints` filtra pontos de curva próximos: distância mínima de `MinWaypointGap` (10, Manhattan) entre waypoints consecutivos (só afeta a representação editável; a caminhada densa executada é preservada).
- Web planeja/edita/recalcula; o client só executa. Web define a rota ativa (`/route/active`); o client lê `routeId` em `/tiles` e busca `/route/entry?id=` (entra pelo ponto mais próximo do jogador).
- Client: switch **Rota** (aba Ativação), lotes de **3**, cursor pela posição real, loop; respeita gate de monstros e cavebot. Monstros bloqueando → apenas pausa e retoma automaticamente (como o Explorar), sem desligar o switch.
- Página **Entorno atual** (web): quando há rota ativa, demarca a rota (linha amarela) e o **caminho comandado no último tick** (linha azul). O client envia `command` (tiles) no payload de `/tiles`; o backend devolve `routeId` e `command` em `/data`.
- Sem rotas órfãs: `DeleteCave` apaga as rotas; só se cria rota para cave `explored`.
- **Auto-rota**: ao concluir a exploração (`/command` com `done`), o client chama `POST /route/auto` (se `Config.autoRoute`, default on). O backend gera `"Rota (auto)"` se não existir (senão usa a mais recente), ativa e o client troca Explorar→Rota. Cave não explorada → `ok:false` + notificação.

### Web UI
- Menu: filtro por nome + **tiles mínimos** (default 50, `localStorage`).
- **Notificações**: sino com badge + banner de 30s (`notices.js`), alimentado por `GET /notices`.

### Visualização (chunks + LOD)
- Chunks fixos (64×64) por nível de LOD; nível `L`: 2^L tiles/célula; `L = max(0, floor(log2(1/scale)))`.
- Carregue só os chunks visíveis; o volume é limitado pela tela, não pelo mapa.
