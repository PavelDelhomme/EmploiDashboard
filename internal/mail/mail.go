package mail

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

func IsSmtpConfigured() bool {
	return strings.TrimSpace(os.Getenv("SMTP_HOST")) != ""
}

func SendTest(to string) error {
	return sendOne(to, "[Emploi Rennes] Test SMTP",
		"Si vous lisez ce message, la configuration SMTP fonctionne.",
		"<p>Si vous lisez ce message, la configuration <strong>SMTP</strong> fonctionne.</p>")
}

func SendNewEvent(rcpt []string, title, when, where, url string) error {
	subj := "[Emploi Rennes] " + title
	txt := strings.Join([]string{title, when, where, url}, "\n")
	html := fmt.Sprintf(`<div style="font-family:system-ui,sans-serif">
<h2>%s</h2>%s%s%s<p style="color:#666;font-size:13px">Alerte — Rennes emploi dashboard</p></div>`,
		escapeHTML(title),
		ifNonEmpty(when, "<p><strong>Date :</strong> "+escapeHTML(when)+"</p>"),
		ifNonEmpty(where, "<p><strong>Lieu :</strong> "+escapeHTML(where)+"</p>"),
		ifNonEmpty(url, `<p><a href="`+escapeAttr(url)+`">Voir / s’inscrire</a></p>`))
	for _, to := range rcpt {
		if strings.TrimSpace(to) == "" {
			continue
		}
		if err := sendOne(strings.TrimSpace(to), subj, txt, html); err != nil {
			return err
		}
	}
	return nil
}

func ifNonEmpty(s, h string) string {
	if s == "" {
		return ""
	}
	return h
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func escapeAttr(s string) string { return strings.ReplaceAll(s, `"`, "&quot;") }

func sendOne(to, subject, text, html string) error {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" {
		return fmt.Errorf("SMTP non configuré")
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("MAIL_FROM")
	if from == "" {
		from = "Rennes Emploi Dashboard <no-reply@localhost>"
	}
	addr := host + ":" + port
	var auth smtp.Auth
	if user != "" && pass != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}
	boundary := "bnd_rennes"
	headers := "From: " + from + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\n"
	headers += "MIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=" + boundary + "\r\n\r\n"
	body := "--" + boundary + "\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + text + "\r\n"
	body += "--" + boundary + "\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + html + "\r\n"
	body += "--" + boundary + "--\r\n"
	msg := []byte(headers + body)
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}
