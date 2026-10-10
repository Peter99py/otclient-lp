const App = {
  caves: [],
  currentPage: null,
  currentCaveId: null,
  data: null,
  rev: null,
  pollTimer: null,
  online: null,

  async init() {
    try {
      const res = await fetch("/caves");
      this.caves = await res.json();
    } catch (e) {
      this.caves = [];
    }
    Menu.init();
    Menu.load();
    Notices.init();
    this.bindZoom();
    this.bindFilter();
    this.bindChrome();
    this.bindKeys();
    this.showPage("surroundings");
    this.startStatusPoll();
  },

  bindChrome() {
    const help = document.getElementById("help-btn");
    const helpPanel = document.getElementById("help-panel");
    if (help) help.onclick = () => { helpPanel.hidden = !helpPanel.hidden; };

    const legend = document.getElementById("legend");
    const lt = document.getElementById("legend-toggle");
    if (lt) {
      lt.onclick = () => {
        legend.classList.toggle("collapsed");
        this.store("legendCollapsed", legend.classList.contains("collapsed"));
      };
      if (this.load("legendCollapsed")) legend.classList.add("collapsed");
    }

    const menu = document.getElementById("menu");
    const mt = document.getElementById("menu-toggle");
    if (mt) {
      mt.onclick = () => {
        menu.classList.toggle("collapsed");
        this.store("menuCollapsed", menu.classList.contains("collapsed"));
        this.relayout();
      };
      if (this.load("menuCollapsed")) menu.classList.add("collapsed");
    }
  },

  store(key, on) {
    try { localStorage.setItem("megabrain." + key, on ? "1" : "0"); } catch (e) {}
  },

  load(key) {
    try { return localStorage.getItem("megabrain." + key) === "1"; } catch (e) { return false; }
  },

  relayout() {
    if (this.currentPage === "surroundings") {
      Surroundings.resize();
    } else if (this.currentPage) {
      MapView.resize();
      MapView.render();
    }
  },

  bindKeys() {
    window.addEventListener("keydown", (e) => {
      if (e.target && /INPUT|SELECT|TEXTAREA/.test(e.target.tagName)) return;
      if (e.key === "+" || e.key === "=") MapView.zoomIn();
      else if (e.key === "-" || e.key === "_") MapView.zoomOut();
      else if (e.key === "Home") MapView.gotoPlayer();
      else if (e.key === "Escape") {
        const hp = document.getElementById("help-panel");
        if (hp && !hp.hidden) { hp.hidden = true; return; }
        const nl = document.getElementById("notif-list");
        if (nl && !nl.hidden) { nl.hidden = true; return; }
      }
    });
  },

  bindFilter() {
    const el = document.getElementById("cave-filter");
    if (el) el.oninput = () => Menu.setFilter(el.value);
    const mt = document.getElementById("cave-min-tiles");
    if (mt) {
      mt.value = Menu.minTiles;
      mt.oninput = () => Menu.setMinTiles(mt.value);
    }
    const ex = document.getElementById("cave-explored");
    if (ex) {
      ex.value = Menu.explored;
      ex.onchange = () => Menu.setExplored(ex.value);
    }
    const so = document.getElementById("cave-sort");
    if (so) {
      so.value = Menu.sort;
      so.onchange = () => Menu.setSort(so.value);
    }
  },

  setOnline(on) {
    if (on === this.online) return;
    this.online = on;
    const el = document.getElementById("conn");
    if (!el) return;
    el.className = on ? "on" : "off";
    el.title = on ? "Conectado ao backend" : "Sem conexão com o backend";
  },

  // Poll adaptativo: 1s no entorno (tempo real), 2s nas caves (só status).
  startStatusPoll() {
    const tick = async () => {
      try {
        const res = await fetch("/data");
        const data = await res.json();
        this.setOnline(true);
        this.data = data;
        this.renderStatus(data);
        if (this.currentPage === "surroundings") Surroundings.onData(data);
        Menu.setCurrentFloor(data && data.player ? data.player.z : null);
        this.updateCurrentCave(data);
        await this.checkRev();
      } catch (e) {
        this.setOnline(false);
      }
      const delay = this.currentPage === "surroundings" ? 1000 : 2000;
      this.pollTimer = setTimeout(tick, delay);
    };
    tick();
  },

  // Compara os contadores de revisão e invalida só o que mudou.
  async checkRev() {
    let rev;
    try {
      const res = await fetch("/rev");
      rev = await res.json();
    } catch (e) {
      return;
    }
    const prev = this.rev;
    this.rev = rev;
    if (!prev) return;
    if (rev.caves !== prev.caves) {
      await this.reloadCaves();
    }
    if (rev.routes !== prev.routes) {
      if (this.currentPage && this.currentPage !== "surroundings") {
        RouteEditor.loadList();
      }
      Surroundings.invalidateRoute();
    }
    if (rev.tiles !== prev.tiles) {
      MapView.invalidateChunks();
    }
  },

  renderStatus(data) {
    const el = document.getElementById("status");
    if (!el) return;
    const count = data && data.monsterCount ? data.monsterCount : 0;
    const paused = !!(data && data.paused);
    const mode = data && data.mode ? data.mode : "off";
    let label, cls;
    if (paused) {
      label = `\u23f8 Pausado \u2014 ${count} monstros`;
      cls = "paused";
    } else if (mode === "route") {
      label = `\u27a4 Rota \u2014 ${count} monstros`;
      cls = "route";
    } else if (mode === "explore") {
      label = `\u25b6 Explorando \u2014 ${count} monstros`;
      cls = "active";
    } else {
      label = "\u25a0 Desligado";
      cls = "off";
    }
    el.textContent = label;
    el.className = cls;
  },

  bindZoom() {
    const zin = document.getElementById("zoom-in");
    const zout = document.getElementById("zoom-out");
    const gp = document.getElementById("goto-player");
    if (zin) zin.onclick = () => MapView.zoomIn();
    if (zout) zout.onclick = () => MapView.zoomOut();
    if (gp) gp.onclick = () => MapView.gotoPlayer();
  },

  async reloadCaves() {
    try {
      const res = await fetch("/caves");
      this.caves = await res.json();
    } catch (e) {
      this.caves = [];
    }
    Menu.load();
    // Se a página atual era a cave apagada, volta para o entorno.
    if (
      this.currentPage &&
      this.currentPage !== "surroundings" &&
      !this.caves.find((x) => x.id === this.currentPage)
    ) {
      this.showPage("surroundings");
    }
  },

  // Destaque da cave atual, a partir do payload já buscado (sem novo fetch).
  updateCurrentCave(data) {
    const p = data && data.player;
    if (!p) return;
    const c = this.caves.find(
      (x) =>
        x.z === p.z &&
        p.x >= x.bounds.min_x &&
        p.x <= x.bounds.max_x &&
        p.y >= x.bounds.min_y &&
        p.y <= x.bounds.max_y
    );
    const id = c ? c.id : null;
    if (id !== this.currentCaveId) {
      this.currentCaveId = id;
      Menu.setCurrent(id);
    }
  },

  showPage(pageId) {
    this.currentPage = pageId;
    Menu.setActive(pageId);
    const zoom = document.getElementById("zoom");
    if (pageId === "surroundings") {
      MapView.stop();
      Surroundings.start();
      if (zoom) zoom.style.display = "none";
    } else {
      Surroundings.stop();
      MapView.start(pageId);
      if (zoom) zoom.style.display = "flex";
    }
  },
};

window.addEventListener("DOMContentLoaded", () => App.init());
window.addEventListener("resize", () => App.relayout());
