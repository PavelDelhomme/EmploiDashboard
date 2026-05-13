const tokenCache = { accessToken: null, expiresAt: 0 };

function portalBase() {
  return (process.env.FT_PORTAL_BASE_URL || "https://mesevenementsemploi.francetravail.fr/mes-evenements-emploi").replace(
    /\/+$/,
    ""
  );
}

function boolEnv(name, def = false) {
  const v = process.env[name];
  if (v === undefined || v === "") return def;
  return ["1", "true", "yes", "on"].includes(String(v).toLowerCase());
}

export function isMockMode() {
  return boolEnv("MOCK_FT", false);
}

export async function getAccessToken() {
  const now = Date.now();
  if (tokenCache.accessToken && now < tokenCache.expiresAt - 30_000) {
    return tokenCache.accessToken;
  }

  const clientId = process.env.FT_CLIENT_ID;
  const clientSecret = process.env.FT_CLIENT_SECRET;
  const scope = process.env.FT_OAUTH_SCOPE || "api_evenementsv1 evenements";
  const tokenUrl =
    process.env.FT_TOKEN_URL ||
    "https://entreprise.francetravail.fr/connexion/oauth2/access_token?realm=/partenaire";

  if (!clientId || !clientSecret) {
    throw new Error("FT_CLIENT_ID / FT_CLIENT_SECRET manquants (ou active MOCK_FT=true pour la démo)");
  }

  const body = new URLSearchParams({
    grant_type: "client_credentials",
    client_id: clientId,
    client_secret: clientSecret,
    scope,
  });

  const res = await fetch(tokenUrl, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });

  const text = await res.text();
  if (!res.ok) {
    throw new Error(`Token FT HTTP ${res.status} — ${text.slice(0, 500)}`);
  }

  let json;
  try {
    json = JSON.parse(text);
  } catch {
    throw new Error(`Réponse token FT non JSON : ${text.slice(0, 200)}`);
  }

  if (!json.access_token) {
    throw new Error(`Token FT sans access_token : ${text.slice(0, 300)}`);
  }

  const ttlMs = Number(json.expires_in || 1200) * 1000;
  tokenCache.accessToken = json.access_token;
  tokenCache.expiresAt = Date.now() + ttlMs;
  return tokenCache.accessToken;
}

function joinApiUrl() {
  const base = (process.env.FT_API_BASE || "https://api.francetravail.io/").replace(/\/?$/, "/");
  const path = (process.env.FT_EVENTS_PATH || "partenaire/evenements/v1/evenements").replace(/^\/+/, "");
  return new URL(path, base);
}

function buildSearchParams(query) {
  const sp = new URLSearchParams();

  const lat = query.lat ?? process.env.RENNES_LAT;
  const lon = query.lon ?? process.env.RENNES_LON;
  const distance = query.radiusKm ?? process.env.DEFAULT_RADIUS_KM ?? "40";

  if (lat) sp.set("latitude", String(lat));
  if (lon) sp.set("longitude", String(lon));
  if (distance) sp.set("distance", String(distance));

  const cp = (process.env.FT_FILTER_CODE_POSTAL || "").trim();
  if (cp) sp.set("codePostal", cp);
  const dep = (process.env.FT_FILTER_DEPARTEMENT || "").trim();
  if (dep) sp.set("codeDepartement", dep);

  if (query.dateFrom) sp.set("dateDebut", String(query.dateFrom));
  if (query.dateTo) sp.set("dateFin", String(query.dateTo));
  if (query.q) sp.set("motsCles", String(query.q));
  if (query.type) sp.set("type", String(query.type));

  const extra = process.env.FT_EXTRA_QUERY;
  if (extra) {
    const e = new URLSearchParams(extra.startsWith("?") ? extra.slice(1) : extra);
    for (const [k, v] of e) sp.set(k, v);
  }

  return sp;
}

function parseContentRange(header) {
  if (!header || typeof header !== "string") return null;
  const m = header.match(/(\d+)\s*-\s*(\d+)\s*\/\s*(\d+)/);
  if (!m) return null;
  return { start: Number(m[1]), end: Number(m[2]), total: Number(m[3]) };
}

function extractEventsArray(json) {
  if (!json || typeof json !== "object") return [];
  if (Array.isArray(json)) return json;
  const keys = [
    "resultats",
    "evenements",
    "evenementsEmploi",
    "listeEvenements",
    "events",
    "content",
    "records",
    "items",
  ];
  for (const k of keys) {
    if (Array.isArray(json[k])) return json[k];
  }
  const emb = json._embedded;
  if (emb && typeof emb === "object") {
    for (const k of keys) {
      if (Array.isArray(emb[k])) return emb[k];
    }
  }
  return [];
}

function pickFirst(obj, keys) {
  for (const k of keys) {
    const v = obj?.[k];
    if (v !== undefined && v !== null && String(v) !== "") return v;
  }
  return "";
}

function toIsoDate(v) {
  if (!v) return "";
  try {
    const d = new Date(v);
    if (Number.isNaN(d.getTime())) return String(v);
    return d.toISOString();
  } catch {
    return String(v);
  }
}

function normalizeLieu(raw) {
  const l = raw?.lieu;
  if (typeof l === "string") return { libelle: l };
  if (l && typeof l === "object") return l;
  return {};
}

/** URL publique alignée sur le portail « Mes événements emploi » (fallback si l’API ne fournit pas de lien). */
function resolvePublicUrl(raw, norm) {
  const direct = pickFirst(raw, ["url", "lien", "urlInscription", "urlDetail", "lienDetail", "lienWeb"]);
  if (direct) return String(direct);

  const tpl = (process.env.FT_PORTAL_EVENT_URL_TEMPLATE || "").trim();
  const portal = portalBase();
  if (tpl) {
    const id =
      pickFirst(raw, ["id", "idEvenement", "identifiant", "identifiantEvenement", "numero", "codeEvenement"]) ||
      norm.key;
    return tpl
      .replace(/\{id\}/g, encodeURIComponent(String(id)))
      .replace(/\{portal\}/g, portal);
  }
  return `${portal}/evenements`;
}

export function normalizeEvent(raw) {
  const id =
    pickFirst(raw, ["id", "idEvenement", "identifiant", "identifiantEvenement", "numero"]) ||
    pickFirst(raw, ["codeEvenement"]);
  const title = pickFirst(raw, ["titre", "intitule", "libelle", "nom", "title"]);
  const startAt = toIsoDate(
    pickFirst(raw, [
      "dateDebut",
      "dateHeureDebut",
      "dateDebutEvenement",
      "date",
      "horaireDebut",
      "startDate",
    ])
  );

  const lieu = normalizeLieu(raw);
  const locationLabel = [
    pickFirst(raw, ["ville", "commune", "libelleCommune"]),
    pickFirst(lieu, ["libelle", "nom", "ville", "commune", "intitule"]),
    pickFirst(raw, ["codePostal", "code_postal"]) || pickFirst(lieu, ["codePostal", "code_postal"]),
  ]
    .filter(Boolean)
    .filter((v, i, a) => a.indexOf(v) === i)
    .join(" ");

  const lat = Number(
    pickFirst(raw, ["latitude"]) ||
      pickFirst(lieu, ["latitude", "lat", "coordLat"]) ||
      NaN
  );
  const lon = Number(
    pickFirst(raw, ["longitude"]) ||
      pickFirst(lieu, ["longitude", "lon", "coordLon"]) ||
      NaN
  );

  const typeLabel = pickFirst(raw, ["type", "typeEvenement", "libelleType", "categorie", "nature"]);

  const key = String(id || `${title}|${startAt}|${locationLabel}`).slice(0, 512);

  const url = resolvePublicUrl(raw, { key, title, startAt, locationLabel });

  return {
    key,
    title: title || "Événement emploi",
    startAt,
    locationLabel: locationLabel || "",
    lat: Number.isFinite(lat) ? lat : null,
    lon: Number.isFinite(lon) ? lon : null,
    url,
    typeLabel: typeLabel || "",
    raw,
  };
}

function dedupeByKey(events) {
  const m = new Map();
  for (const ev of events) {
    if (!m.has(ev.key)) m.set(ev.key, ev);
  }
  return [...m.values()];
}

export async function fetchEventsFromFranceTravail(query) {
  const token = await getAccessToken();
  const baseUrl = joinApiUrl();
  const sp = buildSearchParams(query);
  baseUrl.search = sp.toString();

  const pageSize = Math.max(10, Math.min(500, Number(process.env.FT_RANGE_PAGE_SIZE || 200)));
  const useRange = boolEnv("FT_USE_RANGE_HEADER", false);

  const collect = [];

  const fetchOnce = async (rangeHeader) => {
    const headers = {
      Authorization: `Bearer ${token}`,
      Accept: "application/json",
    };
    if (rangeHeader) headers.Range = rangeHeader;

    const res = await fetch(baseUrl, { headers });
    const text = await res.text();

    if (res.status === 416 && rangeHeader) {
      return { retryWithoutRange: true, res: null, json: null, text: "" };
    }

    if (!res.ok) {
      const err = new Error(`API événements FT HTTP ${res.status} — ${text.slice(0, 400)}`);
      err.status = res.status;
      err.detail = text;
      throw err;
    }

    let json;
    try {
      json = JSON.parse(text);
    } catch {
      throw new Error(`Réponse événements FT non JSON : ${text.slice(0, 200)}`);
    }

    return { retryWithoutRange: false, res, json, text };
  };

  if (!useRange) {
    const { json } = await fetchOnce(null);
    const arr = extractEventsArray(json);
    return dedupeByKey(arr.map(normalizeEvent));
  }

  let start = 0;
  let guard = 0;
  while (guard < 60) {
    guard += 1;
    const end = start + pageSize - 1;
    const rangeHeader = `items=${start}-${end}`;
    let out = await fetchOnce(rangeHeader);
    if (out.retryWithoutRange) {
      const out2 = await fetchOnce(null);
      const arr2 = extractEventsArray(out2.json);
      return dedupeByKey(arr2.map(normalizeEvent));
    }

    const { res, json } = out;
    const arr = extractEventsArray(json);
    collect.push(...arr.map(normalizeEvent));

    const cr = res.headers.get("content-range") || res.headers.get("Content-Range");
    const parsed = parseContentRange(cr);
    if (!parsed || arr.length === 0) break;
    if (parsed.end + 1 >= parsed.total) break;
    start = parsed.end + 1;
  }

  return dedupeByKey(collect);
}

export function mockEventsRennes() {
  const base = new Date();
  const d = (days) => {
    const x = new Date(base);
    x.setDate(x.getDate() + days);
    return x.toISOString();
  };
  const portal = portalBase();
  return [
    {
      key: "mock-forum-rennes-1",
      title: "Forum de l’emploi — Rennes (démo)",
      startAt: d(2),
      locationLabel: "Rennes 35000",
      lat: 48.1173,
      lon: -1.6778,
      url: `${portal}/evenements`,
      typeLabel: "Forum",
      raw: { demo: true },
    },
    {
      key: "mock-jobdating-cesson-1",
      title: "Job dating — Cesson-Sévigné (démo)",
      startAt: d(4),
      locationLabel: "Cesson-Sévigné 35510",
      lat: 48.1192,
      lon: -1.6036,
      url: `${portal}/evenements`,
      typeLabel: "Job dating",
      raw: { demo: true },
    },
    {
      key: "mock-atelier-saintmalo-1",
      title: "Atelier CV — Saint-Malo (démo, hors fenêtre dates)",
      startAt: d(55),
      locationLabel: "Saint-Malo 35400",
      lat: 48.6493,
      lon: -2.0075,
      url: `${portal}/evenements`,
      typeLabel: "Atelier",
      raw: { demo: true },
    },
  ];
}

export async function getEventsForDashboard(query) {
  if (isMockMode()) {
    return { source: "mock", events: mockEventsRennes() };
  }
  const events = await fetchEventsFromFranceTravail(query);
  return { source: "francetravail", events };
}
