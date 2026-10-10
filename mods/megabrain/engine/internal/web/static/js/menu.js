const Menu = {
  filter: "",
  minTiles: 50,
  explored: "all",
  sort: "name",

  init() {
    try {
      const v = localStorage.getItem("megabrain.minTiles");
      if (v !== null) this.minTiles = Math.max(0, Number(v) || 0);
      const e = localStorage.getItem("megabrain.explored");
      if (e) this.explored = e;
      const s = localStorage.getItem("megabrain.sort");
      if (s) this.sort = s;
    } catch (e) {}
  },

  load() {
    const el = document.getElementById("menu-items");
    const frag = document.createDocumentFragment();
    this.addItem(frag, "surroundings", "Entorno atual");

    const byZ = new Map();
    for (const c of App.caves) {
      if (!byZ.has(c.z)) byZ.set(c.z, []);
      byZ.get(c.z).push(c);
    }
    const zs = [...byZ.keys()].sort((a, b) => a - b);
    const q = this.filter.trim().toLowerCase();
    for (const z of zs) {
      let cs = byZ.get(z).filter(
        (c) =>
          (!q || c.name.toLowerCase().includes(q)) &&
          c.tiles >= this.minTiles &&
          (this.explored === "all" ||
            (this.explored === "yes" && c.explored) ||
            (this.explored === "no" && !c.explored))
      );
      cs = this.sortList(cs);
      if (cs.length === 0) continue;
      const header = document.createElement("div");
      header.className = "menu-header";
      header.dataset.z = z;
      header.textContent = `Andar ${z} (${cs.length})`;
      frag.appendChild(header);
      for (const c of cs) this.addCave(frag, c);
    }
    el.replaceChildren(frag);
    this.setCurrentFloor(App.data && App.data.player ? App.data.player.z : null);
    this.setCurrent(App.currentCaveId);
  },

  setFilter(value) {
    this.filter = value;
    this.load();
  },

  setExplored(value) {
    this.explored = value;
    try { localStorage.setItem("megabrain.explored", value); } catch (e) {}
    this.load();
  },

  setSort(value) {
    this.sort = value;
    try { localStorage.setItem("megabrain.sort", value); } catch (e) {}
    this.load();
  },

  sortList(cs) {
    const arr = cs.slice();
    if (this.sort === "tiles") arr.sort((a, b) => b.tiles - a.tiles);
    else if (this.sort === "recent")
      arr.sort((a, b) => (b.updated_at || 0) - (a.updated_at || 0));
    else arr.sort((a, b) => a.name.localeCompare(b.name));
    return arr;
  },

  setMinTiles(value) {
    this.minTiles = Math.max(0, Math.floor(Number(value) || 0));
    try {
      localStorage.setItem("megabrain.minTiles", String(this.minTiles));
    } catch (e) {}
    this.load();
  },

  addItem(el, id, label) {
    const div = document.createElement("div");
    div.className = "menu-item";
    div.dataset.id = id;
    div.textContent = label;
    div.onclick = () => App.showPage(id);
    el.appendChild(div);
  },

  addCave(el, c) {
    const div = document.createElement("div");
    div.className = "menu-item region";
    div.dataset.id = c.id;
    if (App.currentCaveId === c.id) div.classList.add("current");

    const name = document.createElement("span");
    name.className = "region-name";
    name.textContent = c.name;

    const meta = document.createElement("span");
    meta.className = "region-meta";
    meta.textContent = `${c.tiles} ${c.explored ? "✓" : "…"}`;

    const edit = document.createElement("button");
    edit.className = "region-edit";
    edit.title = "Renomear";
    edit.textContent = "✎";
    edit.onclick = (e) => {
      e.stopPropagation();
      this.startRename(div, c);
    };

    const del = document.createElement("button");
    del.className = "region-del";
    del.title = "Apagar cave";
    del.textContent = "🗑";
    del.onclick = (e) => {
      e.stopPropagation();
      this.deleteCave(c);
    };

    div.append(name, meta, edit, del);
    div.onclick = () => App.showPage(c.id);
    el.appendChild(div);
  },

  async deleteCave(c) {
    const ok = confirm(
      `Apagar a cave "${c.name}" (${c.tiles} tiles)?\n\n` +
        "Isso remove os tiles dela do banco de dados e não pode ser desfeito."
    );
    if (!ok) return;
    const again = confirm(`Confirma apagar "${c.name}" definitivamente?`);
    if (!again) return;
    try {
      await fetch("/caves/delete", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: c.id }),
      });
    } catch (e) {}
    await App.reloadCaves();
  },

  startRename(div, c) {
    const input = document.createElement("input");
    input.className = "region-rename";
    input.value = c.name;
    div.replaceChildren(input);
    input.focus();
    input.select();

    let done = false;
    const commit = async (save) => {
      if (done) return;
      done = true;
      const name = input.value.trim();
      if (save && name && name !== c.name) {
        try {
          await fetch("/caves/rename", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ id: c.id, name }),
          });
          c.name = name;
        } catch (e) {}
      }
      this.load();
    };

    input.onblur = () => commit(true);
    input.onkeydown = (e) => {
      if (e.key === "Enter") input.blur();
      else if (e.key === "Escape") {
        input.value = c.name;
        input.blur();
      }
    };
  },

  setActive(id) {
    document.querySelectorAll(".menu-item").forEach((d) => {
      d.classList.toggle("active", d.dataset.id === id);
    });
  },

  setCurrent(id) {
    document.querySelectorAll(".menu-item.region").forEach((d) => {
      d.classList.toggle("current", d.dataset.id === id);
    });
  },

  setCurrentFloor(z) {
    document.querySelectorAll(".menu-header").forEach((h) => {
      h.classList.toggle("current-floor", z != null && Number(h.dataset.z) === z);
    });
  },
};
