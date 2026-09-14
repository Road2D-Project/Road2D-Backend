package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"time"

	"Road-To-Destination-BE/module/share"
)

type Sender interface {
	Send(ctx context.Context, kind Kind, to string, form any) error
	SendHTML(ctx context.Context, to, subject, html string) error
	SendHTMLForm(ctx context.Context, to, subject, htmlTemplate string, form any) error
	SendFile(ctx context.Context, to, subject string, fsys fs.FS, name string, form any) error
}

type Service struct {
	host     string
	port     int
	username string
	password string
	from     string
	fromName string
}

var _ Sender = (*Service)(nil)

func NewFromEnv() *Service {
	port, err := strconv.Atoi(share.GetEnvStringDefault("SMTP_PORT", "587"))
	if err != nil || port <= 0 {
		port = 587
	}
	username := os.Getenv("SMTP_USERNAME")
	from := share.GetEnvStringDefault("SMTP_FROM", username)
	return &Service{
		host:     os.Getenv("SMTP_HOST"),
		port:     port,
		username: username,
		password: os.Getenv("SMTP_PASSWORD"),
		from:     from,
		fromName: share.GetEnvStringDefault("SMTP_FROM_NAME", "Road2D"),
	}
}

func (s *Service) Send(ctx context.Context, kind Kind, to string, form any) error {
	item, err := lookup(kind)
	if err != nil {
		return err
	}
	if item.page {
		return fmt.Errorf("kind %q is a page template; use mail.Render", kind)
	}
	html, err := Render(kind, form)
	if err != nil {
		return err
	}
	return s.SendHTML(ctx, to, item.subject, html)
}

func (s *Service) SendHTMLForm(ctx context.Context, to, subject, htmlTemplate string, form any) error {
	html, err := RenderHTMLForm(htmlTemplate, form)
	if err != nil {
		return err
	}
	return s.SendHTML(ctx, to, subject, html)
}

func (s *Service) SendFile(ctx context.Context, to, subject string, fsys fs.FS, name string, form any) error {
	html, err := RenderFile(fsys, name, form)
	if err != nil {
		return err
	}
	return s.SendHTML(ctx, to, subject, html)
}

func (s *Service) SendHTML(ctx context.Context, to, subject, htmlBody string) error {
	if s.host == "" || s.from == "" {
		return errors.New("smtp is not configured (set SMTP_HOST, SMTP_FROM or SMTP_USERNAME)")
	}
	if to == "" {
		return errors.New("missing mail recipient")
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	dialer := &net.Dialer{}
	if deadline, ok := ctx.Deadline(); ok {
		dialer.Deadline = deadline
	}

	var (
		conn net.Conn
		err  error
	)
	if s.port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.host})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp connect: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if s.port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	if s.username != "" {
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	fromHeader := s.from
	if s.fromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", s.fromName), s.from)
	}

	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\n", fromHeader)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&msg, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&msg, "Content-Type: text/html; charset=UTF-8\r\n")
	fmt.Fprintf(&msg, "Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(&msg)
	if _, err := qp.Write([]byte(htmlBody)); err != nil {
		return err
	}
	if err := qp.Close(); err != nil {
		return err
	}

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("smtp from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp to: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write(msg.Bytes()); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
