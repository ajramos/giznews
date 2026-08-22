package digest

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
)

// fakeSMTP is enough of a server to prove the client speaks the protocol: the
// wire code is the part no unit test of the renderer would ever exercise.
func fakeSMTP(t *testing.T) (addr string, received func() string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	var (
		mu   sync.Mutex
		body strings.Builder
	)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		w := bufio.NewWriter(conn)
		say := func(s string) {
			w.WriteString(s + "\r\n")
			w.Flush()
		}
		say("220 fake ESMTP")
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if inData {
				if line == "." {
					inData = false
					say("250 Ok: queued")
					continue
				}
				mu.Lock()
				body.WriteString(line + "\n")
				mu.Unlock()
				continue
			}
			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				say("250-fake greets you")
				say("250 AUTH PLAIN")
			case strings.HasPrefix(line, "MAIL FROM"), strings.HasPrefix(line, "RCPT TO"):
				say("250 Ok")
			case strings.HasPrefix(line, "AUTH"):
				say("235 authenticated")
			case line == "DATA":
				inData = true
				say("354 go ahead")
			case line == "QUIT":
				say("221 bye")
				return
			default:
				say("250 Ok")
			}
		}
	}()

	return ln.Addr().String(), func() string {
		mu.Lock()
		defer mu.Unlock()
		return body.String()
	}
}

func TestSendDeliversTheDigest(t *testing.T) {
	addr, received := fakeSMTP(t)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	cfg := SMTP{Host: host, From: "me@example.test", To: []string{"me@example.test"}}
	if _, err := parsePort(port, &cfg); err != nil {
		t.Fatal(err)
	}

	page := HTML(sampleDigest())
	if err := Send(cfg, "AI digest · 2026-08-22", page); err != nil {
		t.Fatalf("send: %v", err)
	}

	got := received()
	for _, want := range []string{
		"Subject: AI digest · 2026-08-22",
		"Content-Type: text/html; charset=UTF-8",
		"To: me@example.test",
		"<!doctype html>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the message is missing %q:\n%s", want, got)
		}
	}
}

// Sending is the one thing that leaves the machine: without somewhere to send
// to, it refuses rather than guessing.
func TestSendRefusesWhenNotConfigured(t *testing.T) {
	for _, cfg := range []SMTP{
		{},
		{Host: "smtp.example.test"},       // nobody to send to
		{To: []string{"me@example.test"}}, // nowhere to send from
	} {
		if cfg.Configured() {
			t.Fatalf("%+v should not count as configured", cfg)
		}
		if err := Send(cfg, "subject", "<p>body</p>"); err == nil {
			t.Fatalf("%+v was allowed to send", cfg)
		}
	}
	// Configured but with no sender address is also refused, before dialling.
	err := Send(SMTP{Host: "smtp.example.test", To: []string{"me@example.test"}}, "s", "b")
	if err == nil || !strings.Contains(err.Error(), "sender") {
		t.Fatalf("error = %v, want it to name the missing sender", err)
	}
}

// parsePort keeps the test honest about the port the fake server picked.
func parsePort(port string, cfg *SMTP) (int, error) {
	n := 0
	for _, r := range port {
		n = n*10 + int(r-'0')
	}
	cfg.Port = n
	return n, nil
}
