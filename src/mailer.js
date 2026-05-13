import nodemailer from "nodemailer";

function getTransport() {
  const host = process.env.SMTP_HOST;
  const port = Number(process.env.SMTP_PORT || 587);
  const secure = String(process.env.SMTP_SECURE || "false") === "true";
  const user = process.env.SMTP_USER;
  const pass = process.env.SMTP_PASS;
  if (!host) return null;
  return nodemailer.createTransport({
    host,
    port,
    secure,
    auth: user && pass ? { user, pass } : undefined,
  });
}

export async function sendNewEventEmails({ toAddresses, event }) {
  const transport = getTransport();
  if (!transport || !toAddresses.length) return { skipped: true };

  const from = process.env.MAIL_FROM || "Rennes Emploi Dashboard <no-reply@localhost>";
  const title = event.title || "Nouvel événement emploi";
  const url = event.url || "";
  const when = event.startAt || "";
  const where = event.locationLabel || "";
  const html = `
  <div style="font-family:system-ui,sans-serif;line-height:1.5">
    <h2 style="margin:0 0 12px">${escapeHtml(title)}</h2>
    ${when ? `<p><strong>Date :</strong> ${escapeHtml(when)}</p>` : ""}
    ${where ? `<p><strong>Lieu :</strong> ${escapeHtml(where)}</p>` : ""}
    ${url ? `<p><a href="${escapeAttr(url)}">Voir / s’inscrire</a></p>` : ""}
    <p style="color:#666;font-size:13px">Alerte automatique — Rennes emploi dashboard</p>
  </div>`;

  await transport.sendMail({
    from,
    bcc: toAddresses,
    subject: `[Emploi Rennes] ${title}`,
    text: [title, when, where, url].filter(Boolean).join("\n"),
    html,
  });
  return { sent: toAddresses.length };
}

export async function sendTestEmail(to) {
  const transport = getTransport();
  if (!transport) throw new Error("SMTP non configuré (SMTP_HOST vide)");
  const from = process.env.MAIL_FROM || "Rennes Emploi Dashboard <no-reply@localhost>";
  await transport.sendMail({
    from,
    to,
    subject: "[Emploi Rennes] Test SMTP",
    text: "Si vous lisez ce message, la configuration SMTP fonctionne.",
    html: "<p>Si vous lisez ce message, la configuration <strong>SMTP</strong> fonctionne.</p>",
  });
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function escapeAttr(s) {
  return String(s).replace(/"/g, "&quot;");
}
