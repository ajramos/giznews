package digest

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Sending is the one thing in giznews that leaves the machine, so it is off
// unless it has been configured on purpose, and it never happens on its own:
// something has to ask for it, every time.

// SMTP is where to send a digest. Empty Host or To means "not configured",
// which is the default and is never an error — only an explicit request to
// send without configuration is.
type SMTP struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	// StartTLS upgrades the connection after greeting (port 587). Port 465 is
	// implicit TLS and is detected from the port.
	StartTLS bool `json:"starttls"`
}

// Configured reports whether there is somewhere to send to.
func (s SMTP) Configured() bool {
	return strings.TrimSpace(s.Host) != "" && len(s.To) > 0
}

// Send mails one digest as HTML. The caller has already stored it, so a
// failure here costs the delivery and nothing else.
func Send(cfg SMTP, subject, htmlBody string) error {
	if !cfg.Configured() {
		return fmt.Errorf("digest: no SMTP host or recipient configured")
	}
	port := cfg.Port
	if port == 0 {
		port = 587
	}
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}
	if from == "" {
		return fmt.Errorf("digest: no sender address configured")
	}

	msg := buildMessage(from, cfg.To, subject, htmlBody)
	addr := net.JoinHostPort(cfg.Host, fmt.Sprint(port))

	client, err := dial(addr, cfg.Host, port)
	if err != nil {
		return fmt.Errorf("digest: connect to %s: %w", addr, err)
	}
	defer client.Close()

	if cfg.StartTLS && port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
				return fmt.Errorf("digest: starttls: %w", err)
			}
		}
	}
	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			// Deliberately without the password, or the log becomes the leak.
			return fmt.Errorf("digest: authenticate as %s: %w", cfg.Username, err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("digest: sender rejected: %w", err)
	}
	for _, to := range cfg.To {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("digest: recipient %s rejected: %w", to, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("digest: send: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("digest: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("digest: finish body: %w", err)
	}
	return client.Quit()
}

// dial opens the connection, wrapping it in TLS up front on port 465.
func dial(addr, host string, port int) (*smtp.Client, error) {
	if port == 465 {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, host)
	}
	conn, err := net.DialTimeout("tcp", addr, 20*time.Second)
	if err != nil {
		return nil, err
	}
	return smtp.NewClient(conn, host)
}

// buildMessage writes the headers by hand: the body is HTML the renderer
// already produced, and there is nothing to negotiate.
func buildMessage(from string, to []string, subject, htmlBody string) string {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(strings.ReplaceAll(htmlBody, "\n", "\r\n"))
	return b.String()
}
