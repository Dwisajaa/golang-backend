package mailer

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
)

// SMTPMailer sends via net/smtp (STARTTLS). It is the production transport;
// credentials come from config, never from code.
type SMTPMailer struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
}

func NewSMTPMailer(host string, port int, username, password, fromAddress, fromName string) *SMTPMailer {
	return &SMTPMailer{
		Host: host, Port: port, Username: username, Password: password,
		FromAddress: fromAddress, FromName: fromName,
	}
}

func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	addr := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))

	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}

	from := m.FromAddress
	if m.FromName != "" {
		from = fmt.Sprintf("%s <%s>", m.FromName, m.FromAddress)
	}

	header := strings.Join([]string{
		"From: " + from,
		"To: " + msg.ToEmail,
		"Subject: " + msg.Subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
	}, "\r\n") + "\r\n\r\n" + msg.Body

	// net/smtp.SendMail negotiates STARTTLS when the server offers it.
	return smtp.SendMail(addr, auth, m.FromAddress, []string{msg.ToEmail}, []byte(header))
}
