"""
Service HTTP optionnel : Camoufox + extraction d’événements Schema.org (JSON-LD @type Event).
À utiliser en complément de l’API officielle — respecter les CGU du site cible (SCRAPER_TARGET_URL).
"""

from __future__ import annotations

import asyncio
import hashlib
import json
import logging
import os
import re
import time
from concurrent.futures import ThreadPoolExecutor
from typing import Any

from camoufox.sync_api import Camoufox
from fastapi import FastAPI, Query

logging.basicConfig(level=logging.INFO)
log = logging.getLogger("camoufox-scraper")

executor = ThreadPoolExecutor(max_workers=1, thread_name_prefix="camoufox")

app = FastAPI(title="Camoufox event scraper", version="0.1.0")


def _is_event_type(t: Any) -> bool:
    if t == "Event":
        return True
    if isinstance(t, list):
        return "Event" in t
    return False


def _loc_label(loc: Any) -> str:
    if loc is None:
        return ""
    if isinstance(loc, str):
        return loc.strip()
    if not isinstance(loc, dict):
        return ""
    if "name" in loc and isinstance(loc["name"], str):
        return loc["name"].strip()
    addr = loc.get("address")
    if isinstance(addr, dict):
        parts = [
            addr.get("streetAddress"),
            addr.get("postalCode"),
            addr.get("addressLocality"),
        ]
        return ", ".join(str(p) for p in parts if p)
    if isinstance(addr, str):
        return addr.strip()
    return ""


def _event_url(item: dict[str, Any]) -> str:
    u = item.get("url")
    if isinstance(u, str) and u.strip():
        return u.strip()
    if isinstance(u, list) and u:
        x = u[0]
        return x.strip() if isinstance(x, str) else ""
    return ""


def _stable_key(item: dict[str, Any]) -> str:
    title = (item.get("name") or item.get("headline") or "")[:200]
    url = _event_url(item)
    raw = f"{url}|{title}"
    h = hashlib.sha256(raw.encode("utf-8")).hexdigest()[:20]
    slug = re.sub(r"[^a-z0-9]+", "-", title.lower())[:40].strip("-") or "evt"
    return f"scrape-{slug}-{h}"


def _ld_items_to_events(data: Any) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    if isinstance(data, dict) and "@graph" in data:
        items = data["@graph"]
    elif isinstance(data, list):
        items = data
    else:
        items = [data]
    for item in items:
        if not isinstance(item, dict):
            continue
        if not _is_event_type(item.get("@type")):
            continue
        title = item.get("name") or item.get("headline") or ""
        if not isinstance(title, str) or not title.strip():
            continue
        start = item.get("startDate") or item.get("startTime") or ""
        if not isinstance(start, str):
            start = str(start) if start else ""
        ev = {
            "key": _stable_key(item),
            "title": title.strip(),
            "startAt": start.strip(),
            "locationLabel": _loc_label(item.get("location")),
            "url": _event_url(item),
            "typeLabel": "Scraping",
        }
        out.append(ev)
    return out


def scrape_sync() -> list[dict[str, Any]]:
    target = os.environ.get("SCRAPER_TARGET_URL", "").strip()
    if not target:
        log.info("SCRAPER_TARGET_URL vide — aucun scraping (0 événement)")
        return []

    timeout_ms = int(os.environ.get("SCRAPER_GOTO_TIMEOUT_MS", "90000"))

    with Camoufox(headless=True, locale="fr-FR,fr") as browser:
        page = browser.new_page()
        page.goto(target, wait_until="domcontentloaded", timeout=timeout_ms)
        time.sleep(float(os.environ.get("SCRAPER_SETTLE_SEC", "2")))

        scripts = page.locator('script[type="application/ld+json"]').all()
        events: list[dict[str, Any]] = []
        for script in scripts:
            raw = script.inner_text().strip()
            if not raw:
                continue
            try:
                data = json.loads(raw)
            except json.JSONDecodeError:
                continue
            if isinstance(data, list):
                for chunk in data:
                    events.extend(_ld_items_to_events(chunk))
            else:
                events.extend(_ld_items_to_events(data))
        log.info("JSON-LD: %d événements extraits", len(events))
        return events


@app.get("/health")
def health() -> dict[str, str]:
    return {"ok": "true"}


@app.get("/events")
async def events(
    lat: str = "",
    lon: str = "",
    radius_km: str = "",
    from_date: str = Query("", alias="from"),
    to: str = "",
) -> dict[str, Any]:
    # Les paramètres sont transmis pour alignement avec le dashboard ; le scraping
    # utilise surtout SCRAPER_TARGET_URL (page listant ou détail).
    _ = (lat, lon, radius_km, from_date, to)
    loop = asyncio.get_event_loop()
    try:
        rows = await loop.run_in_executor(executor, scrape_sync)
    except Exception as e:
        log.exception("scrape_sync")
        return {"events": [], "error": str(e), "source": "camoufox"}

    return {"events": rows, "source": "camoufox-jsonld"}
