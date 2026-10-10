// Editor de rotas: mostra a rota da cave selecionada e permite editar os
// pontos (adicionar / mover / remover). A web planeja; o client só executa.
const RouteEditor = {
  active: false,
  caveId: null,
  caveDbId: 0,
  z: 7,
  routes: [],
  current: null,
  waypoints: [],
  dirty: false,
  mode: "pan",
  selected: -1,
  dragging: false,
  dragIndex: -1,

  async open(caveId) {
    this.active = true;
    this.caveId = caveId;
    const c = App.caves.find((x) => x.id === caveId);
    this.z = c ? c.z : 7;
    this.caveDbId = c ? Number(caveId.split(":")[1]) : 0;
    this.current = null;
    this.waypoints = [];
    this.dirty = false;
    this.selected = -1;
    this.setMode("pan");
    document.getElementById("route-panel").hidden = false;
    this.bind();
    await this.loadList();
  },

  close() {
    this.active = false;
    const p = document.getElementById("route-panel");
    if (p) p.hidden = true;
  },

  bind() {
    if (this._bound) return;
    this._bound = true;
    document.getElementById("route-select").onchange = (e) => {
      const id = Number(e.target.value);
      if (id) this.select(id);
      else this.clearCurrent();
    };
    document.getElementById("route-new").onclick = () => this.newRoute();
    document.getElementById("route-rename").onclick = () => this.rename();
    document.getElementById("route-del").onclick = () => this.deleteRoute();
    document.getElementById("route-help").onclick = () => {
      const el = document.getElementById("route-help-text");
      el.hidden = !el.hidden;
    };
    document.getElementById("route-recalc").onclick = () => this.recalc();
    document.getElementById("route-save").onclick = () => this.save();
    document.getElementById("route-run").onclick = () => this.run();
    document.getElementById("route-stop").onclick = () => this.stop();
    document.querySelectorAll("#route-modes button").forEach((b) => {
      b.onclick = () => this.setMode(b.dataset.mode);
    });
  },

  async loadList() {
    try {
      const res = await fetch(`/routes?cave=${this.caveDbId}`);
      this.routes = res.ok ? await res.json() : [];
    } catch (e) {
      this.routes = [];
    }
    let activeId = 0;
    try {
      const a = await fetch("/route/active");
      if (a.ok) activeId = (await a.json()).id || 0;
    } catch (e) {}
    this.activeId = activeId;
    const sel = document.getElementById("route-select");
    sel.innerHTML = "";
    if (this.routes.length === 0) {
      const o = document.createElement("option");
      o.value = "";
      o.textContent = "(sem rotas)";
      sel.appendChild(o);
    } else {
      for (const r of this.routes) {
        const o = document.createElement("option");
        o.value = r.id;
        o.textContent =
          (r.id === activeId ? "\u25b6 " : "") + `${r.name} (${r.length})`;
        if (r.id === activeId) o.className = "active-route";
        sel.appendChild(o);
      }
    }
    if (this.current) sel.value = this.current.id;
    this.updateInfo();
  },

  async select(id) {
    try {
      const res = await fetch(`/route?id=${id}`);
      if (!res.ok) return;
      this.current = await res.json();
      this.waypoints = this.current.waypoints.map((w) => [w[0], w[1]]);
      this.dirty = false;
      this.selected = -1;
      MapView.render();
    } catch (e) {}
    this.updateInfo();
  },

  clearCurrent() {
    this.current = null;
    this.waypoints = [];
    this.dirty = false;
    this.selected = -1;
    MapView.render();
    this.updateInfo();
  },

  async newRoute() {
    const name = prompt("Nome da rota:", "Rota");
    if (name === null) return;
    try {
      const res = await fetch("/routes", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ cave: this.caveId, name }),
      });
      if (!res.ok) {
        alert("Não foi possível criar a rota: " + (await res.text()));
        return;
      }
      const view = await res.json();
      await this.loadList();
      document.getElementById("route-select").value = view.id;
      await this.select(view.id);
    } catch (e) {}
  },

  async rename() {
    if (!this.current) return;
    const name = prompt("Nome da rota:", this.current.name);
    if (name === null) return;
    const trimmed = name.trim();
    if (!trimmed || trimmed === this.current.name) return;
    try {
      const res = await fetch("/routes/rename", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: this.current.id, name: trimmed }),
      });
      if (!res.ok) return;
      this.current = await res.json();
    } catch (e) {}
    await this.loadList();
    this.updateInfo();
  },

  async deleteRoute() {
    if (!this.current) return;
    if (!confirm(`Apagar a rota "${this.current.name}"?`)) return;
    try {
      await fetch("/routes/delete", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: this.current.id }),
      });
    } catch (e) {}
    this.clearCurrent();
    await this.loadList();
  },

  async recalc() {
    if (!this.current) return;
    if (this.dirty && !confirm("Descartar alterações e recalcular?")) return;
    try {
      const res = await fetch("/routes/recalc", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: this.current.id }),
      });
      if (!res.ok) {
        alert("Falha ao recalcular: " + (await res.text()));
        return;
      }
      this.current = await res.json();
      this.waypoints = this.current.waypoints.map((w) => [w[0], w[1]]);
      this.dirty = false;
      this.selected = -1;
      MapView.render();
    } catch (e) {}
    this.updateInfo();
  },

  async save() {
    if (!this.current || !this.dirty) return;
    try {
      const res = await fetch("/routes/update", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          id: this.current.id,
          name: this.current.name,
          waypoints: this.waypoints,
        }),
      });
      const data = await res.json();
      if (!res.ok || data.ok === false) {
        alert("Falha ao salvar: " + (data.error || res.status));
        return;
      }
      this.current = data;
      this.waypoints = data.waypoints.map((w) => [w[0], w[1]]);
      this.dirty = false;
      this.selected = -1;
      MapView.render();
      await this.loadList();
    } catch (e) {}
    this.updateInfo();
  },

  async run() {
    if (!this.current) return;
    if (this.dirty) await this.save();
    try {
      await fetch("/route/active", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: this.current.id }),
      });
    } catch (e) {}
    await this.loadList();
    this.flash("Rota ativa.");
  },

  async stop() {
    try {
      await fetch("/route/active/clear", { method: "POST" });
    } catch (e) {}
    await this.loadList();
    this.flash("Rota parada.");
  },

  setMode(m) {
    this.mode = m;
    document.querySelectorAll("#route-modes button").forEach((b) => {
      b.classList.toggle("active", b.dataset.mode === m);
    });
  },

  updateInfo() {
    const el = document.getElementById("route-info");
    if (!el) return;
    if (!this.current) {
      el.textContent = "Sem rota.";
      return;
    }
    const steps = this.dirty ? "?" : this.current.length;
    el.textContent =
      `${this.current.name} | ${this.waypoints.length} pontos | ${steps} passos` +
      (this.dirty ? " (não salvo)" : "");
  },

  flash(msg) {
    const el = document.getElementById("route-info");
    if (!el) return;
    el.textContent = msg;
    setTimeout(() => this.updateInfo(), 1500);
  },

  // ---- desenho ----

  draw(ctx) {
    if (!this.active || !this.current) return;
    const scale = MapView.scale;
    const ox = MapView.originX;
    const oy = MapView.originY;
    const toS = (x, y) => [(x - ox) * scale, (y - oy) * scale];

    if (!this.dirty && this.current.path && this.current.path.length > 1) {
      ctx.strokeStyle = "#ffd54f";
      ctx.lineWidth = Math.max(1, scale * 0.3);
      ctx.beginPath();
      const p = this.current.path;
      for (let i = 0; i < p.length; i++) {
        const [sx, sy] = toS(p[i][0], p[i][1]);
        if (i === 0) ctx.moveTo(sx, sy);
        else ctx.lineTo(sx, sy);
      }
      ctx.stroke();
    }

    const wps = this.waypoints;
    if (this.dirty && wps.length > 1) {
      ctx.strokeStyle = "#ffd54f";
      ctx.setLineDash([4, 4]);
      ctx.lineWidth = 1;
      ctx.beginPath();
      for (let i = 0; i < wps.length; i++) {
        const [sx, sy] = toS(wps[i][0], wps[i][1]);
        if (i === 0) ctx.moveTo(sx, sy);
        else ctx.lineTo(sx, sy);
      }
      ctx.stroke();
      ctx.setLineDash([]);
    }

    const r = Math.max(3, Math.min(8, scale * 0.6));
    for (let i = 0; i < wps.length; i++) {
      const [sx, sy] = toS(wps[i][0], wps[i][1]);
      ctx.beginPath();
      ctx.arc(sx, sy, r, 0, Math.PI * 2);
      ctx.fillStyle = i === this.selected ? "#ffffff" : "#ffd54f";
      ctx.fill();
      ctx.strokeStyle = "#000";
      ctx.lineWidth = 1;
      ctx.stroke();
    }
  },

  // ---- interação ----

  eventTile(e) {
    const rect = MapView.canvas.getBoundingClientRect();
    const sx = e.clientX - rect.left;
    const sy = e.clientY - rect.top;
    return {
      x: Math.floor(MapView.originX + sx / MapView.scale),
      y: Math.floor(MapView.originY + sy / MapView.scale),
    };
  },

  nearestWaypoint(x, y) {
    let best = -1;
    let bestD = Infinity;
    for (let i = 0; i < this.waypoints.length; i++) {
      const d =
        Math.abs(this.waypoints[i][0] - x) + Math.abs(this.waypoints[i][1] - y);
      if (d < bestD) {
        bestD = d;
        best = i;
      }
    }
    const thr = Math.max(2, Math.ceil(10 / MapView.scale));
    return bestD <= thr ? best : -1;
  },

  onCanvasDown(e) {
    if (!this.current) return false;
    const { x, y } = this.eventTile(e);
    if (this.mode === "add") {
      this.addAt(x, y);
      return true;
    }
    if (this.mode === "move") {
      const idx = this.nearestWaypoint(x, y);
      if (idx >= 0) {
        this.dragging = true;
        this.dragIndex = idx;
        this.selected = idx;
        MapView.render();
      }
      return true;
    }
    if (this.mode === "remove") {
      const idx = this.nearestWaypoint(x, y);
      if (idx >= 0) this.removeAt(idx);
      return true;
    }
    return false;
  },

  onCanvasMove(e) {
    if (!this.dragging) return;
    const { x, y } = this.eventTile(e);
    this.waypoints[this.dragIndex] = [x, y];
    this.dirty = true;
    MapView.render();
  },

  async onCanvasUp() {
    if (!this.dragging) return;
    this.dragging = false;
    const idx = this.dragIndex;
    this.dragIndex = -1;
    const wp = this.waypoints[idx];
    const snapped = await this.snapWalkable(wp[0], wp[1]);
    if (snapped) this.waypoints[idx] = snapped;
    this.dirty = true;
    MapView.render();
    this.updateInfo();
  },

  async addAt(x, y) {
    const snapped = await this.snapWalkable(x, y);
    if (!snapped) {
      this.flash("Tile não andável.");
      return;
    }
    const [wx, wy] = snapped;
    if (this.waypoints.length === 0) {
      this.waypoints.push([wx, wy]);
    } else {
      let bestI = this.waypoints.length;
      let bestD = Infinity;
      for (let i = 0; i < this.waypoints.length; i++) {
        const a = this.waypoints[i];
        const b = this.waypoints[(i + 1) % this.waypoints.length];
        const d = distToSegment(wx, wy, a, b);
        if (d < bestD) {
          bestD = d;
          bestI = i;
        }
      }
      this.waypoints.splice(bestI + 1, 0, [wx, wy]);
    }
    this.dirty = true;
    MapView.render();
    this.updateInfo();
  },

  removeAt(idx) {
    this.waypoints.splice(idx, 1);
    this.dirty = true;
    this.selected = -1;
    MapView.render();
    this.updateInfo();
  },

  async snapWalkable(x, y) {
    const x0 = x - 2;
    const y0 = y - 2;
    try {
      const res = await fetch(`/known?z=${this.z}&x=${x0}&y=${y0}&w=5&h=5`);
      if (!res.ok) return null;
      const data = await res.json();
      let best = null;
      let bestD = Infinity;
      for (const t of data.tiles || []) {
        if (!t.walkable) continue;
        const d = Math.abs(t.x - x) + Math.abs(t.y - y);
        if (d < bestD) {
          bestD = d;
          best = [t.x, t.y];
        }
      }
      return best;
    } catch (e) {
      return null;
    }
  },
};

function distToSegment(px, py, a, b) {
  const ax = a[0];
  const ay = a[1];
  const dx = b[0] - ax;
  const dy = b[1] - ay;
  const len2 = dx * dx + dy * dy;
  let t = len2 === 0 ? 0 : ((px - ax) * dx + (py - ay) * dy) / len2;
  t = Math.max(0, Math.min(1, t));
  const cx = ax + t * dx;
  const cy = ay + t * dy;
  return Math.abs(px - cx) + Math.abs(py - cy);
}
