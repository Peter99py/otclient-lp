// Notificações recentes (em memória no backend). Sino com badge + banner de 30s.
const Notices = {
  items: [],
  seenMax: null, // maior id já visto (badge de não lidas)
  bannerMax: null, // maior id já exibido no banner
  bannerTimer: null,

  init() {
    const bell = document.getElementById("notif-bell");
    if (bell) bell.onclick = () => this.toggle();
    this.poll();
    setInterval(() => this.poll(), 1000);
  },

  async poll() {
    try {
      const res = await fetch("/notices");
      if (!res.ok) return;
      this.items = (await res.json()) || [];
    } catch (e) {
      return;
    }
    const maxId = this.items.length ? this.items[this.items.length - 1].id : 0;
    if (this.seenMax === null) {
      this.seenMax = maxId;
      this.bannerMax = maxId;
    }
    if (maxId > this.bannerMax) {
      this.bannerMax = maxId;
      this.showBanner(this.items[this.items.length - 1].message);
    }
    this.renderBadge();
    const list = document.getElementById("notif-list");
    if (list && !list.hidden) this.renderList();
  },

  renderBadge() {
    const badge = document.getElementById("notif-badge");
    if (!badge) return;
    const unread = this.items.filter((n) => n.id > this.seenMax).length;
    if (unread > 0) {
      badge.textContent = unread;
      badge.hidden = false;
    } else {
      badge.hidden = true;
    }
  },

  showBanner(msg) {
    const el = document.getElementById("notice-banner");
    if (!el) return;
    el.textContent = msg;
    el.hidden = false;
    if (this.bannerTimer) clearTimeout(this.bannerTimer);
    this.bannerTimer = setTimeout(() => {
      el.hidden = true;
    }, 30000);
  },

  toggle() {
    const list = document.getElementById("notif-list");
    if (!list) return;
    list.hidden = !list.hidden;
    if (!list.hidden) {
      this.renderList();
      this.seenMax = this.items.length
        ? this.items[this.items.length - 1].id
        : 0;
      this.renderBadge();
    }
  },

  renderList() {
    const list = document.getElementById("notif-list");
    if (!list) return;
    if (!this.items.length) {
      list.innerHTML = '<div class="notif-empty">Sem notificações.</div>';
      return;
    }
    list.innerHTML = "";
    for (let i = this.items.length - 1; i >= 0; i--) {
      const n = this.items[i];
      const div = document.createElement("div");
      div.className = "notif-item";
      const msg = document.createElement("div");
      msg.className = "notif-msg";
      msg.textContent = n.message;
      const time = document.createElement("div");
      time.className = "notif-time";
      time.textContent = this.relTime(n.at);
      div.append(msg, time);
      list.appendChild(div);
    }
  },

  relTime(at) {
    const d = Math.max(0, Math.floor(Date.now() / 1000 - at));
    if (d < 60) return d + "s";
    if (d < 3600) return Math.floor(d / 60) + "min";
    return Math.floor(d / 3600) + "h";
  },
};
