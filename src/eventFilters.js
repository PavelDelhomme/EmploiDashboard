/** Filtres appliqués côté serveur (l’API FT peut ignorer certains paramètres). */

export function eventInDateRange(ev, dateFrom, dateTo) {
  if (!dateFrom && !dateTo) return true;
  if (!ev.startAt) return true;
  const d = new Date(ev.startAt);
  if (Number.isNaN(d.getTime())) return true;
  if (dateFrom) {
    const t0 = new Date(`${dateFrom}T00:00:00.000`);
    if (d < t0) return false;
  }
  if (dateTo) {
    const t1 = new Date(`${dateTo}T23:59:59.999`);
    if (d > t1) return false;
  }
  return true;
}

export function eventMatchesTypeSlug(ev, type) {
  if (!type) return true;
  const hay = `${ev.typeLabel || ""} ${ev.title || ""}`.toLowerCase();
  if (type === "forum") return hay.includes("forum") || hay.includes("salon");
  if (type === "job") return hay.includes("job") || hay.includes("dating");
  if (type === "atelier") return hay.includes("atelier") || hay.includes("réunion");
  return true;
}

export function filterEvents(events, { dateFrom, dateTo, type }) {
  return events.filter(
    (ev) => eventInDateRange(ev, dateFrom, dateTo) && eventMatchesTypeSlug(ev, type)
  );
}
