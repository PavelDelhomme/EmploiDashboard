import "dotenv/config";
import express from "express";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { getDb, addSubscriber, removeSubscriber, listSubscribers } from "./db.js";
import { filterEvents } from "./eventFilters.js";
import { getEventsForDashboard } from "./francetravail.js";
import { isRecentlySeen, pollOnce, startPoller } from "./poller.js";
import { sendTestEmail } from "./mailer.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(__dirname, "..");

const app = express();
app.use(express.json());
app.use(express.static(path.join(root, "public")));

app.get("/api/health", (_req, res) => {
  res.json({ ok: true, ts: new Date().toISOString() });
});

app.get("/api/config", (_req, res) => {
  let subscriberCount = 0;
  try {
    subscriberCount = listSubscribers(getDb()).length;
  } catch {
    /* sqlite indisponible au premier instant */
  }
  res.json({
    center: {
      lat: Number(process.env.RENNES_LAT || 48.1173),
      lon: Number(process.env.RENNES_LON || -1.6778),
    },
    radiusKm: Number(process.env.DEFAULT_RADIUS_KM || 40),
    mockMode: String(process.env.MOCK_FT || "").toLowerCase() === "true",
    subscriberCount,
  });
});

app.get("/api/events", async (req, res) => {
  try {
    const db = getDb();
    const { source, events: rawEvents } = await getEventsForDashboard({
      lat: process.env.RENNES_LAT,
      lon: process.env.RENNES_LON,
      radiusKm: process.env.DEFAULT_RADIUS_KM,
      dateFrom: req.query.from,
      dateTo: req.query.to,
      q: req.query.q,
      type: req.query.type,
    });

    let events = filterEvents(rawEvents, {
      dateFrom: req.query.from,
      dateTo: req.query.to,
      type: req.query.type,
    });

    const qkw = String(req.query.q || "")
      .trim()
      .toLowerCase();
    if (qkw) {
      events = events.filter((ev) => {
        const blob = `${ev.title} ${ev.locationLabel} ${ev.typeLabel}`.toLowerCase();
        return blob.includes(qkw);
      });
    }

    const enriched = events.map((ev) => {
      const { raw: _raw, ...pub } = ev;
      const { isNewBadge } = isRecentlySeen(db, ev.key);
      return { ...pub, isNew: isNewBadge };
    });

    res.json({ source, count: enriched.length, events: enriched });
  } catch (e) {
    console.error(e);
    res.status(502).json({
      error: e.message || String(e),
      hint:
        "Vérifie FT_CLIENT_ID/SECRET, FT_OAUTH_SCOPE et FT_EVENTS_PATH sur francetravail.io — ou active MOCK_FT=true.",
    });
  }
});

app.get("/api/subscribers", (_req, res) => {
  const db = getDb();
  res.json({ subscribers: listSubscribers(db) });
});

app.post("/api/subscribers", (req, res) => {
  try {
    const db = getDb();
    const email = addSubscriber(db, req.body?.email);
    res.json({ ok: true, email });
  } catch (e) {
    res.status(400).json({ error: e.message || String(e) });
  }
});

app.delete("/api/subscribers/:email", (req, res) => {
  const db = getDb();
  removeSubscriber(db, decodeURIComponent(req.params.email));
  res.json({ ok: true });
});

app.post("/api/test-mail", async (req, res) => {
  try {
    const to = String(req.body?.email || "").trim();
    if (!to) return res.status(400).json({ error: "email requis" });
    await sendTestEmail(to);
    res.json({ ok: true });
  } catch (e) {
    res.status(500).json({ error: e.message || String(e) });
  }
});

app.post("/api/poll-once", async (_req, res) => {
  try {
    const r = await pollOnce();
    res.json(r);
  } catch (e) {
    res.status(500).json({ error: e.message || String(e) });
  }
});

const port = Number(process.env.PORT || 3000);
app.listen(port, "0.0.0.0", () => {
  console.log(`HTTP http://0.0.0.0:${port}`);
  startPoller();
  if (String(process.env.STARTUP_POLL || "true").toLowerCase() !== "false") {
    setTimeout(() => {
      pollOnce().then(console.log).catch(console.error);
    }, 3000);
  }
});
