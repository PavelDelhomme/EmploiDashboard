const errEl = document.getElementById("err");
const listEl = document.getElementById("list");
const sourceTag = document.getElementById("source-tag");
const subMsg = document.getElementById("sub-msg");
const subsContainer = document.getElementById("subs-container");
const loadingEl = document.getElementById("loading");
const btnRefresh = document.getElementById("btn-refresh");
const statusLine = document.getElementById("status-line");

let uiPollTimer = null;
let map;
let layerMarkers;
let layerZone;
let markers = {};
let activeKey = null;

function fmtDate(iso) {
  if (!iso) return "";
  try {
    const d = new Date(iso);
    return d.toLocaleString("fr-FR", { dateStyle: "medium", timeStyle: "short" });
  } catch {
    return iso;
  }
}

async function loadConfig() {
  const r = await fetch("/api/config");
  return r.json();
}

async function loadSubs() {
  const r = await fetch("/api/subscribers");
  const j = await r.json();
  subsContainer.innerHTML = "";
  if (!j.subscribers?.length) {
    subsContainer.textContent = "Aucun abonné pour l’instant.";
    return;
  }
  j.subscribers.forEach((s) => {
    const row = document.createElement("div");
    row.className = "sub-row";
    const em = escapeHtml(s.email);
    row.innerHTML = `<span>${em}</span>`;
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "ghost";
    btn.textContent = "Retirer";
    btn.addEventListener("click", async (e) => {
      e.preventDefault();
      e.stopPropagation();
      await fetch("/api/subscribers/" + encodeURIComponent(s.email), { method: "DELETE" });
      await loadSubs();
      const cfg = await loadConfig();
      updateSourceTag(cfg);
    });
    row.appendChild(btn);
    subsContainer.appendChild(row);
  });
}

function updateSourceTag(cfg) {
  const base = cfg.mockMode
    ? "Source : données de démo (MOCK_FT=true)"
    : "Source : API France Travail (événements)";
  const n = Number(cfg.subscriberCount || 0);
  sourceTag.textContent = n > 0 ? `${base} · ${n} abonné(s) email` : base;
}

function setErr(t) {
  errEl.textContent = t || "";
}

async function loadEvents() {
  setErr("");
  const q = new URLSearchParams();
  const qv = document.getElementById("q").value.trim();
  const type = document.getElementById("type").value;
  const from = document.getElementById("from").value;
  const to = document.getElementById("to").value;
  if (qv) q.set("q", qv);
  if (type) q.set("type", type);
  if (from) q.set("from", from);
  if (to) q.set("to", to);
  const r = await fetch("/api/events?" + q.toString());
  const j = await r.json();
  if (!r.ok) throw new Error(j.error || j.hint || "Erreur chargement");
  return j;
}

async function loadApiStatus() {
  try {
    const r = await fetch("/api/status");
    const s = await r.json();
    if (!r.ok) return;
    const ft = s.franceTravail || {};
    const poll = s.lastPoll || {};
    let line = "";
    if (s.mockMode) line = "API : mode démo · ";
    else if (ft.tokenOk) line = "API France Travail : jeton OK · ";
    else if (!ft.credentialsSet) line = "API : identifiants FT manquants · ";
    else line = "API : jeton refusé (" + (ft.error || "?").slice(0, 80) + ") · ";
    line += s.smtpConfigured ? "SMTP configuré" : "SMTP non configuré";
    if (poll.at) {
      line +=
        " · Dernier poll : " +
        (poll.ok ? "OK" : "erreur") +
        " (" +
        new Date(poll.at).toLocaleString("fr-FR") +
        ")";
    }
    statusLine.textContent = line;
  } catch {
    statusLine.textContent = "";
  }
}

function renderList(events) {
  listEl.innerHTML = "";
  if (!events.length) {
    const p = document.createElement("p");
    p.className = "empty-list";
    p.textContent =
      "Aucun événement pour ces filtres. Élargis les dates ou les critères, ou vérifie la config API (voir la ligne d’état ci-dessus).";
    listEl.appendChild(p);
    return;
  }
  events.forEach((ev) => {
    const div = document.createElement("div");
    div.className = "card" + (ev.key === activeKey ? " active" : "");
    div.dataset.key = ev.key;
    div.innerHTML = `
      <h3>${escapeHtml(ev.title)}${
      ev.isNew ? '<span class="badge">NOUVEAU</span>' : ""
    }</h3>
      <div class="meta">${escapeHtml(fmtDate(ev.startAt))}</div>
      <div class="meta">${escapeHtml(ev.locationLabel || "")}</div>
      <div class="meta">${escapeHtml(ev.typeLabel || "")}</div>
      ${
        ev.url
          ? `<div class="meta"><a href="${escapeAttr(ev.url)}" target="_blank" rel="noopener" onclick="event.stopPropagation()">Détail / inscription</a></div>`
          : ""
      }
    `;
    div.addEventListener("click", () => selectEvent(ev));
    listEl.appendChild(div);
  });
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function selectEvent(ev) {
  activeKey = ev.key;
  document.querySelectorAll(".card").forEach((c) => {
    c.classList.toggle("active", c.dataset.key === ev.key);
  });
  if (ev.lat != null && ev.lon != null) {
    map.setView([ev.lat, ev.lon], 12, { animate: true });
    const m = markers[ev.key];
    if (m) m.openPopup();
  } else if (ev.url) {
    window.open(ev.url, "_blank", "noopener");
  }
}

let eventsRef = [];

async function refresh() {
  loadingEl.classList.add("visible");
  btnRefresh.disabled = true;
  try {
    const cfg = await loadConfig();
    updateSourceTag(cfg);

    const j = await loadEvents();
    eventsRef = j.events || [];
    renderList(eventsRef);
    layerMarkers.clearLayers();
    markers = {};
    const bounds = [];
    eventsRef.forEach((ev) => {
      if (ev.lat == null || ev.lon == null) return;
      const m = L.marker([ev.lat, ev.lon]);
      const popup = `<strong>${escapeHtml(ev.title)}</strong><br/>${escapeHtml(
        fmtDate(ev.startAt)
      )}<br/><a href="${escapeAttr(ev.url)}" target="_blank" rel="noopener">Détails</a>`;
      m.bindPopup(popup);
      m.addTo(layerMarkers);
      markers[ev.key] = m;
      bounds.push([ev.lat, ev.lon]);
    });
    if (bounds.length) map.fitBounds(bounds, { padding: [32, 32], maxZoom: 11 });
  } catch (e) {
    setErr(e.message || String(e));
  } finally {
    loadingEl.classList.remove("visible");
    btnRefresh.disabled = false;
  }
  loadApiStatus();
}

function escapeAttr(s) {
  return String(s).replace(/"/g, "&quot;");
}

async function initMap() {
  const cfg = await loadConfig();
  map = L.map("map", { zoomControl: true }).setView([cfg.center.lat, cfg.center.lon], 10);
  L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
    maxZoom: 19,
    attribution: "&copy; OpenStreetMap",
  }).addTo(map);
  layerZone = L.layerGroup().addTo(map);
  L.circle([cfg.center.lat, cfg.center.lon], {
    radius: (cfg.radiusKm || 40) * 1000,
    color: "#3b82f6",
    weight: 1,
    fillColor: "#3b82f6",
    fillOpacity: 0.08,
  }).addTo(layerZone);
  layerMarkers = L.layerGroup().addTo(map);
}

document.getElementById("btn-refresh").addEventListener("click", refresh);
document.getElementById("btn-apply").addEventListener("click", refresh);

document.getElementById("btn-sub").addEventListener("click", async () => {
  subMsg.textContent = "";
  const email = document.getElementById("sub-email").value.trim();
  const r = await fetch("/api/subscribers", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  });
  const j = await r.json();
  if (!r.ok) subMsg.innerHTML = `<span class="err">${escapeHtml(j.error)}</span>`;
  else subMsg.innerHTML = `<span class="ok">Abonnement enregistré : ${escapeHtml(j.email)}</span>`;
  await loadSubs();
  const cfg = await loadConfig();
  updateSourceTag(cfg);
});

document.getElementById("btn-test").addEventListener("click", async () => {
  subMsg.textContent = "";
  const email = document.getElementById("sub-email").value.trim();
  const r = await fetch("/api/test-mail", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  });
  const j = await r.json();
  if (!r.ok) subMsg.innerHTML = `<span class="err">${escapeHtml(j.error)}</span>`;
  else subMsg.innerHTML = `<span class="ok">Mail de test envoyé.</span>`;
});

document.getElementById("btn-poll").addEventListener("click", async () => {
  subMsg.textContent = "";
  const r = await fetch("/api/poll-once", { method: "POST" });
  const j = await r.json();
  if (!r.ok) subMsg.innerHTML = `<span class="err">${escapeHtml(j.error)}</span>`;
  else subMsg.innerHTML = `<span class="ok">${escapeHtml(JSON.stringify(j))}</span>`;
  await refresh();
});

(async () => {
  const today = new Date();
  const fri = new Date(today);
  const dow = fri.getDay();
  const add = (5 - dow + 7) % 7;
  fri.setDate(fri.getDate() + add);
  document.getElementById("to").value = fri.toISOString().slice(0, 10);
  document.getElementById("from").value = today.toISOString().slice(0, 10);
  await initMap();
  await loadSubs();
  await refresh();
  const cfgPoll = await loadConfig();
  const ms = Number(cfgPoll.uiPollIntervalMs || 0);
  if (ms > 5000) {
    uiPollTimer = setInterval(() => {
      refresh().catch(console.error);
    }, ms);
  }
})();
