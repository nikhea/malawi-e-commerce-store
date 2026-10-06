// Package mail sends transactional email over SMTP. Text-only v1:
// receipts, OTP codes, reset links. HTML templates plug in later without
// changing callers (Message gains the field, SMTP renders multipart).
package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

// Message is one outbound email. To is a single recipient (loop for
// bulk); Subject/Body are pre-rendered by the caller.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender delivers messages. Interface-kept so workers test with a fake.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// SMTPSender delivers via SMTP+STARTTLS (Gmail app passwords work with
// PlainAuth here). Constructed once at wiring from EMAIL_* config.
type SMTPSender struct {
	host     string
	port     int
	username string
	password string
	from     string
	timeout  time.Duration
}

func NewSMTPSender(host string, port int, username, password, from string) *SMTPSender {
	return &SMTPSender{host: host, port: port, username: username, password: password, from: from, timeout: 15 * time.Second}
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if msg.To == "" || msg.Subject == "" || msg.Body == "" {
		return fmt.Errorf("mail: to, subject and body are required")
	}

	dialer := &net.Dialer{Timeout: s.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", s.host, s.port))
	if err != nil {
		return fmt.Errorf("mail: dial: %w", err)
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("mail: client: %w", err)
	}
	defer client.Quit()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
			return fmt.Errorf("mail: starttls: %w", err)
		}
	}
	if ok, _ := client.Extension("AUTH"); ok {
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("mail: to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	body := "From: " + s.from + "\r\n" +
		"To: " + msg.To + "\r\n" +
		"Subject: " + msg.Subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		msg.Body + "\r\n"
	if _, err := w.Write([]byte(body)); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: close: %w", err)
	}
	return nil
}
