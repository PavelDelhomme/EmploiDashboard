const tokenCache = { accessToken: null, expiresAt: 0 };

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

function extractEventsArray(json) {
  if (!json || typeof json !== "object") return [];
  if (Array.isArray(json)) return json;
  const keys = ["resultats", "evenements", "events", "content", "records", "items"];
  for (const k of keys) {
    if (Array.isArray(json[k])) return json[k];
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

export function normalizeEvent(raw) {
  const id =
    pickFirst(raw, ["id", "idEvenement", "identifiant", "identifiantEvenement", "numero"]) ||
    pickFirst(raw, ["codeEvenement"]);
  const title = pickFirst(raw, ["titre", "intitule", "libelle", "nom", "title"]);
  const url = pickFirst(raw, ["url", "lien", "urlInscription", "urlDetail", "lienDetail"]);
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

  const lieu = raw?.lieu || raw?.adresse || {};
  const locationLabel = [
    pickFirst(lieu, ["libelle", "nom", "ville", "commune", "intitule"]),
    pickFirst(lieu, ["codePostal", "code_postal"]),
  ]
    .filter(Boolean)
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

  const typeLabel = pickFirst(raw, ["type", "typeEvenement", "libelleType", "categorie"]);

  const key = String(id || `${title}|${startAt}|${locationLabel}`).slice(0, 512);

  return {
    key,
    title: title || "Événement emploi",
    startAt,
    locationLabel: locationLabel || "",
    lat: Number.isFinite(lat) ? lat : null,
    lon: Number.isFinite(lon) ? lon : null,
    url: url || "",
    typeLabel: typeLabel || "",
    raw,
  };
}

export async function fetchEventsFromFranceTravail(query) {
  const token = await getAccessToken();
  const url = joinApiUrl();
  const sp = new URLSearchParams();

  const lat = query.lat ?? process.env.RENNES_LAT;
  const lon = query.lon ?? process.env.RENNES_LON;
  const distance = query.radiusKm ?? process.env.DEFAULT_RADIUS_KM ?? "40";

  if (lat) sp.set("latitude", String(lat));
  if (lon) sp.set("longitude", String(lon));
  if (distance) sp.set("distance", String(distance));

  if (query.dateFrom) sp.set("dateDebut", String(query.dateFrom));
  if (query.dateTo) sp.set("dateFin", String(query.dateTo));
  if (query.q) sp.set("motsCles", String(query.q));
  if (query.type) sp.set("type", String(query.type));

  const extra = process.env.FT_EXTRA_QUERY;
  if (extra) {
    const e = new URLSearchParams(extra.startsWith("?") ? extra.slice(1) : extra);
    for (const [k, v] of e) sp.set(k, v);
  }

  url.search = sp.toString();

  const res = await fetch(url, {
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/json",
    },
  });

  const text = await res.text();
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

  const arr = extractEventsArray(json);
  return arr.map(normalizeEvent);
}

export function mockEventsRennes() {
  const base = new Date();
  const d = (days) => {
    const x = new Date(base);
    x.setDate(x.getDate() + days);
    return x.toISOString();
  };
  return [
    {
      key: "mock-forum-rennes-1",
      title: "Forum de l’emploi — Rennes (démo)",
      startAt: d(2),
      locationLabel: "Rennes 35000",
      lat: 48.1173,
      lon: -1.6778,
      url: "https://mesevenementsemploi.francetravail.fr/",
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
      url: "https://mesevenementsemploi.francetravail.fr/",
      typeLabel: "Job dating",
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
