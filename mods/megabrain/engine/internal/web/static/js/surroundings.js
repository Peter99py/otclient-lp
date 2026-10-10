const Surroundings = {
  CELL: 20,
  path: null,
  known: null,
  knownKey: null,
  routeId: 0,
  routePath: null,
  routeZ: null,
  busy: false,

  start() {
    this.canvas = document.getElementById("canvas");
    this.ctx = this.canvas.getContext("2d");
    this.resize();
    this.path = null;
    this.known = null;
    this.knownKey = null;
    if (App.data) this.onData(App.data);
  },

  stop() {
    this.path = null;
    this.known = null;
    this.knownKey = null;
  },

  // Força a re-busca da rota ativa (chamado quando o backend sinaliza mudança).
  invalidateRoute() {
    this.routeId = 0;
    this.routePath = null;
    this.routeZ = null;
  },

  resize() {
    if (!this.canvas) return;
    this.canvas.width = this.canvas.clientWidth;
    this.canvas.height = this.canvas.clientHeight;
    this.knownKey = null;
  },

  async onData(data) {
    if (this.busy) return;
    if (!data || !data.tiles || data.tiles.length === 0) return;
    this.busy = true;
    try {
      await this.fetchRoute(data.routeId);
      if (data.routeId) {
        this.path = null;
      } else {
        const pathRes = await fetch("/path");
        this.path = pathRes.ok ? await pathRes.json() : null;
      }
      this.known = await this.fetchKnown(data.player);
      if (App.currentPage !== "surroundings") return;
      this.draw(data);
    } catch (e) {
      document.getElementById("info").textContent = "Erro: " + e;
    } finally {
      this.busy = false;
    }
  },

  // Busca a rota ativa (quando muda) para demarcá-la na página.
  async fetchRoute(routeId) {
    if (!routeId) {
      this.routeId = 0;
      this.routePath = null;
      this.routeZ = null;
      return;
    }
    if (routeId === this.routeId) return;
    this.routeId = routeId;
    this.routePath = null;
    this.routeZ = null;
    try {
      const res = await fetch(`/route?id=${routeId}`);
      const r = res.ok ? await res.json() : null;
      if (!r) return;
      this.routePath = r.path;
      const cave = App.caves.find(
        (c) => Number(c.id.split(":")[1]) === r.cave
      );
      this.routeZ = cave ? cave.z : null;
    } catch (e) {}
  },

  async fetchKnown(player) {
    if (!player) return null;
    const cols = Math.ceil(this.canvas.width / this.CELL) + 2;
    const rows = Math.ceil(this.canvas.height / this.CELL) + 2;
    const x = player.x - Math.floor(cols / 2);
    const y = player.y - Math.floor(rows / 2);
    const key = `${player.z}:${x}:${y}:${cols}:${rows}`;
    if (key === this.knownKey) return this.known;
    try {
      const res = await fetch(
        `/known?z=${player.z}&x=${x}&y=${y}&w=${cols}&h=${rows}`
      );
      if (!res.ok) return this.known;
      const data = await res.json();
      this.knownKey = key;
      return data.tiles || null;
    } catch (e) {
      return this.known;
    }
  },

  draw(data) {
    const ctx = this.ctx;
    const CELL = this.CELL;
    const tiles = data.tiles;

    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const t of tiles) {
      if (t.x < minX) minX = t.x;
      if (t.x > maxX) maxX = t.x;
      if (t.y < minY) minY = t.y;
      if (t.y > maxY) maxY = t.y;
    }

    const toPx = (x, y) => ({
      px: (x - minX) * CELL,
      py: (y - minY) * CELL,
    });

    // Tiles alcançáveis a partir do jogador (ignora o que está após a fronteira).
    const reach = new Set();
    for (const t of tiles) {
      if (t.reachable !== false) reach.add(t.x + "," + t.y);
    }

    ctx.fillStyle = "#1e1e1e";
    ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);

    const p = data.player;
    let offX = 0, offY = 0;
    if (p) {
      offX = this.canvas.width / 2 - ((p.x - minX) * CELL + CELL / 2);
      offY = this.canvas.height / 2 - ((p.y - minY) * CELL + CELL / 2);
    }

    ctx.save();
    ctx.translate(offX, offY);

    if (this.known) {
      ctx.globalAlpha = 0.4;
      for (const t of this.known) {
        const { px, py } = toPx(t.x, t.y);
        ctx.fillStyle = t.walkable ? "#2ecc71" : "#e74c3c";
        ctx.fillRect(px, py, CELL - 1, CELL - 1);
      }
      ctx.globalAlpha = 1;
    }

    for (const t of tiles) {
      if (!reach.has(t.x + "," + t.y)) continue;
      const { px, py } = toPx(t.x, t.y);
      ctx.fillStyle = t.walkable ? "#2ecc71" : "#e74c3c";
      ctx.fillRect(px, py, CELL - 1, CELL - 1);
    }

    this.drawPath(minX, minY, CELL);
    if (p && this.routeZ === p.z) this.drawRoute(minX, minY, CELL);

    if (data.creatures) {
      for (const c of data.creatures) {
        if (!reach.has(c.x + "," + c.y)) continue;
        this.drawCreature(c, minX, minY, CELL);
      }
    }

    if (p) {
      const { px, py } = toPx(p.x, p.y);
      ctx.fillStyle = "#3498db";
      ctx.fillRect(px, py, CELL - 1, CELL - 1);
    }

    this.drawCommand(data.command, minX, minY, CELL);

    ctx.restore();

    if (p) {
      document.getElementById("info").textContent =
        `Jogador: (${p.x}, ${p.y}, ${p.z}) | Tiles: ${tiles.length}`;
    }
  },

  drawPath(minX, minY, CELL) {
    const plan = this.path;
    if (!plan || !plan.path || plan.path.length < 2) return;
    const ctx = this.ctx;

    ctx.strokeStyle = "#00e5ff";
    ctx.lineWidth = 2;
    ctx.beginPath();
    for (let i = 0; i < plan.path.length; i++) {
      const px = (plan.path[i][0] - minX) * CELL + CELL / 2;
      const py = (plan.path[i][1] - minY) * CELL + CELL / 2;
      if (i === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    }
    ctx.stroke();

    if (plan.goal) {
      const gx = (plan.goal[0] - minX) * CELL + CELL / 2;
      const gy = (plan.goal[1] - minY) * CELL + CELL / 2;
      ctx.strokeStyle = "#ffffff";
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.arc(gx, gy, CELL / 2 - 2, 0, Math.PI * 2);
      ctx.stroke();
    }
  },

  drawCreature(c, minX, minY, CELL) {
    const ctx = this.ctx;
    const cx = (c.x - minX) * CELL + CELL / 2;
    const cy = (c.y - minY) * CELL + CELL / 2;
    const colors = { monster: "#ff3b30", npc: "#f1c40f", player: "#3498db" };
    ctx.fillStyle = colors[c.kind] || "#ff3b30";
    ctx.beginPath();
    ctx.arc(cx, cy, CELL / 2 - 3, 0, Math.PI * 2);
    ctx.fill();
  },

  // Demarca a rota ativa (linha amarela).
  drawRoute(minX, minY, CELL) {
    if (!this.routePath || this.routePath.length < 2) return;
    const ctx = this.ctx;
    ctx.strokeStyle = "#ffd54f";
    ctx.lineWidth = 2;
    ctx.beginPath();
    for (let i = 0; i < this.routePath.length; i++) {
      const px = (this.routePath[i][0] - minX) * CELL + CELL / 2;
      const py = (this.routePath[i][1] - minY) * CELL + CELL / 2;
      if (i === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    }
    ctx.stroke();
  },

  // Caminho comandado no último tick (linha azul).
  drawCommand(cmd, minX, minY, CELL) {
    if (!cmd || cmd.length < 2) return;
    const ctx = this.ctx;
    ctx.strokeStyle = "#00e5ff";
    ctx.lineWidth = 3;
    ctx.beginPath();
    for (let i = 0; i < cmd.length; i++) {
      const px = (cmd[i][0] - minX) * CELL + CELL / 2;
      const py = (cmd[i][1] - minY) * CELL + CELL / 2;
      if (i === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    }
    ctx.stroke();
  },
};
