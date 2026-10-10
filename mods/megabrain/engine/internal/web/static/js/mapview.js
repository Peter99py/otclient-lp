const MapView = {
  CHUNK_SIZE: 64,
  MAX_LOD: 4,
  MIN_SCALE: 0.25,
  MAX_SCALE: 16,

  active: false,
  region: null,
  caveDbId: 0,
  z: 7,
  scale: 4,
  originX: 0,
  originY: 0,
  chunks: new Map(),
  chunkImages: new Map(),
  pending: new Set(),
  dragging: false,
  lastMouse: null,
  _raf: null,

  start(caveId) {
    this.stop();
    this.active = true;
    this.region = caveId;
    this.canvas = document.getElementById("canvas");
    this.ctx = this.canvas.getContext("2d");
    this.chunks.clear();
    this.chunkImages.clear();
    this.pending.clear();
    this.updateLoading();

    const c = App.caves.find((x) => x.id === caveId);
    this.z = c ? c.z : 7;
    this.caveDbId = c ? Number(c.id.split(":")[1]) : 0;

    this.resize();
    this.centerOn(c);
    this.bindEvents();
    RouteEditor.open(caveId);
    this.render();
  },

  stop() {
    this.active = false;
    this.pending.clear();
    this.updateLoading();
    this.unbindEvents();
    RouteEditor.close();
  },

  // Chamado quando o backend sinaliza que os tiles mudaram.
  invalidateChunks() {
    this.chunks.clear();
    this.chunkImages.clear();
    if (this.active) this.scheduleRender();
  },

  // Coalesce múltiplos pedidos de desenho num único frame.
  scheduleRender() {
    if (this._raf) return;
    this._raf = requestAnimationFrame(() => {
      this._raf = null;
      this.render();
    });
  },

  resize() {
    if (!this.canvas) return;
    this.canvas.width = this.canvas.clientWidth;
    this.canvas.height = this.canvas.clientHeight;
  },

  centerOn(region) {
    if (!region) return;
    const b = region.bounds;
    const pad = 1;
    const w = b.max_x - b.min_x + 1 + pad * 2;
    const h = b.max_y - b.min_y + 1 + pad * 2;
    const sx = this.canvas.width / w;
    const sy = this.canvas.height / h;
    this.scale = Math.max(
      this.MIN_SCALE,
      Math.min(this.MAX_SCALE, Math.min(sx, sy))
    );
    const cx = (b.min_x + b.max_x) / 2;
    const cy = (b.min_y + b.max_y) / 2;
    this.originX = cx - this.canvas.width / 2 / this.scale;
    this.originY = cy - this.canvas.height / 2 / this.scale;
  },

  lodLevel() {
    if (this.scale >= 1) return 0;
    return Math.min(this.MAX_LOD, Math.floor(Math.log2(1 / this.scale)));
  },

  setScale(s, center) {
    if (!this.canvas) return;
    s = Math.max(this.MIN_SCALE, Math.min(this.MAX_SCALE, s));
    if (center) {
      const cx = this.originX + (this.canvas.width / 2) / this.scale;
      const cy = this.originY + (this.canvas.height / 2) / this.scale;
      this.scale = s;
      this.originX = cx - (this.canvas.width / 2) / this.scale;
      this.originY = cy - (this.canvas.height / 2) / this.scale;
    } else {
      this.scale = s;
    }
    this.scheduleRender();
  },

  // Zoom mantendo fixo o ponto (mx, my) em pixels do canvas.
  setScaleAt(s, mx, my) {
    if (!this.canvas) return;
    s = Math.max(this.MIN_SCALE, Math.min(this.MAX_SCALE, s));
    const tx = this.originX + mx / this.scale;
    const ty = this.originY + my / this.scale;
    this.scale = s;
    this.originX = tx - mx / this.scale;
    this.originY = ty - my / this.scale;
    this.scheduleRender();
  },

  zoomIn() { this.setScale(this.scale * 1.2, true); },
  zoomOut() { this.setScale(this.scale / 1.2, true); },

  // Centraliza no jogador (se estiver no mesmo andar desta cave).
  gotoPlayer() {
    if (!this.active || !this.canvas) return;
    const p = App.data && App.data.player;
    if (!p) return;
    if (p.z !== this.z) {
      document.getElementById("info").textContent =
        `Jogador no andar ${p.z} (esta cave é z=${this.z}).`;
      return;
    }
    this.originX = p.x - this.canvas.width / 2 / this.scale;
    this.originY = p.y - this.canvas.height / 2 / this.scale;
    this.scheduleRender();
  },

  // Tipo do tile a partir do chunk já carregado (sem fetch).
  tileAt(tx, ty) {
    const L = this.lodLevel();
    const cellTiles = 2 ** L;
    const span = this.CHUNK_SIZE * cellTiles;
    const cx = Math.floor(tx / span);
    const cy = Math.floor(ty / span);
    const chunk = this.chunks.get(`${L}:${cx}:${cy}`);
    if (!chunk) return null;
    const i = Math.floor((tx - cx * span) / cellTiles);
    const j = Math.floor((ty - cy * span) / cellTiles);
    const v = chunk.data[j * chunk.size + i];
    if (v === "1") return "andável";
    if (v === "2") return "bloqueado";
    return "desconhecido";
  },

  updateTooltip(e) {
    const tip = document.getElementById("tooltip");
    if (!tip) return;
    const rect = this.canvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;
    if (mx < 0 || my < 0 || mx > rect.width || my > rect.height) {
      tip.hidden = true;
      return;
    }
    const tx = Math.floor(this.originX + mx / this.scale);
    const ty = Math.floor(this.originY + my / this.scale);
    const kind = this.tileAt(tx, ty);
    tip.textContent = `(${tx}, ${ty}, ${this.z})` + (kind ? ` \u2014 ${kind}` : "");
    tip.hidden = false;
    tip.style.left = mx + 12 + "px";
    tip.style.top = my + 12 + "px";
  },

  updateLoading() {
    const el = document.getElementById("loading");
    if (el) el.hidden = this.pending.size === 0;
  },

  bindEvents() {
    this._onDown = (e) => {
      if (RouteEditor.active && RouteEditor.onCanvasDown(e)) return;
      this.dragging = true;
      this.lastMouse = { x: e.clientX, y: e.clientY };
    };
    this._onUp = () => {
      if (RouteEditor.active && RouteEditor.dragging) RouteEditor.onCanvasUp();
      this.dragging = false;
    };
    this._onMove = (e) => {
      this.updateTooltip(e);
      if (RouteEditor.active && RouteEditor.dragging) {
        RouteEditor.onCanvasMove(e);
        return;
      }
      if (!this.dragging) return;
      const dx = e.clientX - this.lastMouse.x;
      const dy = e.clientY - this.lastMouse.y;
      this.lastMouse = { x: e.clientX, y: e.clientY };
      this.originX -= dx / this.scale;
      this.originY -= dy / this.scale;
      this.scheduleRender();
    };
    this._onLeave = () => {
      const tip = document.getElementById("tooltip");
      if (tip) tip.hidden = true;
    };
    this._onWheel = (e) => {
      e.preventDefault();
      const rect = this.canvas.getBoundingClientRect();
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;
      const factor = e.deltaY < 0 ? 1.2 : 1 / 1.2;
      this.setScaleAt(this.scale * factor, mx, my);
    };
    this.canvas.addEventListener("mousedown", this._onDown);
    window.addEventListener("mouseup", this._onUp);
    window.addEventListener("mousemove", this._onMove);
    this.canvas.addEventListener("mouseleave", this._onLeave);
    this.canvas.addEventListener("wheel", this._onWheel, { passive: false });
  },

  unbindEvents() {
    if (!this.canvas) return;
    this.canvas.removeEventListener("mousedown", this._onDown);
    window.removeEventListener("mouseup", this._onUp);
    window.removeEventListener("mousemove", this._onMove);
    this.canvas.removeEventListener("mouseleave", this._onLeave);
    this.canvas.removeEventListener("wheel", this._onWheel);
  },

  render() {
    if (!this.active || !this.ctx) return;
    const ctx = this.ctx;
    const L = this.lodLevel();
    const cellTiles = 2 ** L;
    const cellPx = this.scale * cellTiles;
    const span = this.CHUNK_SIZE * cellTiles;

    ctx.fillStyle = "#1e1e1e";
    ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);

    const minTileX = this.originX;
    const minTileY = this.originY;
    const maxTileX = this.originX + this.canvas.width / this.scale;
    const maxTileY = this.originY + this.canvas.height / this.scale;

    const cx0 = Math.floor(minTileX / span);
    const cx1 = Math.floor(maxTileX / span);
    const cy0 = Math.floor(minTileY / span);
    const cy1 = Math.floor(maxTileY / span);

    for (let cy = cy0; cy <= cy1; cy++) {
      for (let cx = cx0; cx <= cx1; cx++) {
        this.drawChunk(L, cx, cy, cellPx, span);
      }
    }

    RouteEditor.draw(ctx);

    document.getElementById("info").textContent =
      `Cave ${this.region} | zoom ${this.scale.toFixed(2)} | LOD ${L}`;
  },

  drawChunk(L, cx, cy, cellPx, span) {
    const key = `${L}:${cx}:${cy}`;
    const chunk = this.chunks.get(key);
    if (chunk === undefined) {
      this.requestChunk(L, cx, cy);
      return;
    }
    if (chunk === null) return;

    let img = this.chunkImages.get(key);
    if (!img) {
      img = this.buildChunkImage(chunk);
      this.chunkImages.set(key, img);
    }

    const baseX = cx * span;
    const baseY = cy * span;
    const px = Math.ceil(cellPx);
    this.ctx.imageSmoothingEnabled = false;
    this.ctx.drawImage(
      img,
      (baseX - this.originX) * this.scale,
      (baseY - this.originY) * this.scale,
      px * chunk.size,
      px * chunk.size
    );
  },

  // Constrói a imagem do chunk uma única vez (ImageData) e a reutiliza.
  buildChunkImage(chunk) {
    const size = chunk.size;
    const c = document.createElement("canvas");
    c.width = size;
    c.height = size;
    const cx = c.getContext("2d");
    const img = cx.createImageData(size, size);
    const d = img.data;
    for (let j = 0; j < size; j++) {
      for (let i = 0; i < size; i++) {
        const v = chunk.data[j * size + i];
        if (v === "0") continue;
        const idx = (j * size + i) * 4;
        if (v === "1") {
          d[idx] = 46; d[idx + 1] = 204; d[idx + 2] = 113; d[idx + 3] = 255;
        } else {
          d[idx] = 231; d[idx + 1] = 76; d[idx + 2] = 60; d[idx + 3] = 255;
        }
      }
    }
    cx.putImageData(img, 0, 0);
    return c;
  },

  async requestChunk(L, cx, cy) {
    const key = `${L}:${cx}:${cy}`;
    if (this.pending.has(key)) return;
    this.pending.add(key);
    this.updateLoading();
    try {
      const res = await fetch(`/chunk?z=${this.z}&level=${L}&cx=${cx}&cy=${cy}&cave=${this.caveDbId}`);
      this.chunks.set(key, res.ok ? await res.json() : null);
      this.scheduleRender();
    } catch (e) {
      this.chunks.set(key, null);
    } finally {
      this.pending.delete(key);
      this.updateLoading();
    }
  },
};
