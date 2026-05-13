import fs from "node:fs";
import path from "node:path";
import Database from "better-sqlite3";

const dataDir = process.env.DATA_DIR || path.join(process.cwd(), "data");
const dbPath = path.join(dataDir, "app.sqlite");

export function getDb() {
  fs.mkdirSync(dataDir, { recursive: true });
  const db = new Database(dbPath);
  db.pragma("journal_mode = WAL");
  db.exec(`
    CREATE TABLE IF NOT EXISTS seen_events (
      event_key TEXT PRIMARY KEY,
      first_seen_at TEXT NOT NULL
    );
    CREATE TABLE IF NOT EXISTS subscribers (
      email TEXT PRIMARY KEY,
      created_at TEXT NOT NULL
    );
  `);
  return db;
}

export function isNewEvent(db, eventKey) {
  const row = db.prepare("SELECT 1 FROM seen_events WHERE event_key = ?").get(eventKey);
  return !row;
}

export function markEventSeen(db, eventKey, atIso) {
  const at = atIso || new Date().toISOString();
  db.prepare(
    "INSERT OR IGNORE INTO seen_events (event_key, first_seen_at) VALUES (?, ?)"
  ).run(eventKey, at);
}

export function listSubscribers(db) {
  return db.prepare("SELECT email, created_at FROM subscribers ORDER BY created_at ASC").all();
}

export function addSubscriber(db, email) {
  const e = String(email).trim().toLowerCase();
  if (!e || !e.includes("@")) throw new Error("Email invalide");
  db.prepare("INSERT OR IGNORE INTO subscribers (email, created_at) VALUES (?, ?)").run(
    e,
    new Date().toISOString()
  );
  return e;
}

export function removeSubscriber(db, email) {
  const e = String(email).trim().toLowerCase();
  db.prepare("DELETE FROM subscribers WHERE email = ?").run(e);
}
