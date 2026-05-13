import cron from "node-cron";
import { getDb, isNewEvent, markEventSeen, listSubscribers } from "./db.js";
import { sendNewEventEmails } from "./mailer.js";
import { getEventsForDashboard, isMockMode } from "./francetravail.js";

const NEW_BADGE_HOURS = 36;

export function isRecentlySeen(db, eventKey) {
  const row = db.prepare("SELECT first_seen_at FROM seen_events WHERE event_key = ?").get(eventKey);
  if (!row) return { seen: false, isNewBadge: false };
  const t = new Date(row.first_seen_at).getTime();
  const isNewBadge = Date.now() - t < NEW_BADGE_HOURS * 3600 * 1000;
  return { seen: true, isNewBadge };
}

export async function pollOnce() {
  const db = getDb();
  const { events, source } = await getEventsForDashboard({
    lat: process.env.RENNES_LAT,
    lon: process.env.RENNES_LON,
    radiusKm: process.env.DEFAULT_RADIUS_KM,
  });

  const totalSeen = db.prepare("SELECT COUNT(*) AS c FROM seen_events").get().c;
  const silentSeed =
    totalSeen === 0 &&
    events.length > 0 &&
    String(process.env.FT_SEED_SILENT || "true").toLowerCase() !== "false";

  const subs = listSubscribers(db).map((r) => r.email);
  const newOnes = [];
  const seedStamp = "2000-01-01T00:00:00.000Z";

  for (const ev of events) {
    if (!isNewEvent(db, ev.key)) continue;
    if (silentSeed) {
      markEventSeen(db, ev.key, seedStamp);
      continue;
    }
    newOnes.push(ev);
    markEventSeen(db, ev.key);
  }

  if (silentSeed) {
    return {
      source,
      scanned: events.length,
      silentSeed: true,
      message:
        "Premier passage : les événements actuels sont enregistrés sans email (évite le spam). Les prochains nouveaux déclencheront une notification.",
    };
  }

  if (!silentSeed && newOnes.length && subs.length) {
    for (const ev of newOnes) {
      try {
        await sendNewEventEmails({ toAddresses: subs, event: ev });
      } catch (e) {
        console.error("Erreur envoi mail pour", ev.key, e);
      }
    }
  }

  return {
    source,
    scanned: events.length,
    newCount: silentSeed ? 0 : newOnes.length,
    notified: silentSeed ? 0 : subs.length ? newOnes.length : 0,
    subscribers: subs.length,
  };
}

export function startPoller() {
  const expr = process.env.POLL_CRON || "*/30 * * * *";
  const job = cron.schedule(expr, async () => {
    try {
      const r = await pollOnce();
      console.log("[poll]", new Date().toISOString(), r);
    } catch (e) {
      console.error("[poll] erreur", e);
    }
  });
  if (isMockMode()) {
    console.log("[poll] MOCK_FT=true — données fictives, emails réels si SMTP OK.");
  }
  console.log("[poll] cron:", expr);
  return job;
}
