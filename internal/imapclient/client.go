package imapclient

import (
	"context"
	"crypto/tls"
	"fmt"

	goimapclient "github.com/emersion/go-imap/v2/imapclient"
)

const defaultGmailIMAPAddr = "imap.gmail.com:993"

// Client wraps a go-imap/v2 client with Gmail-specific defaults.
type Client struct {
	addr     string
	email    string
	password string
	conn     *goimapclient.Client
}

// New creates a new IMAP Client targeting Gmail.
func New(email, password string) *Client {
	return &Client{
		addr:     defaultGmailIMAPAddr,
		email:    email,
		password: password,
	}
}

// Connect establishes a TLS connection and logs in.
func (c *Client) Connect(_ context.Context) error {
	conn, err := goimapclient.DialTLS(c.addr, &goimapclient.Options{
		TLSConfig: &tls.Config{},
	})
	if err != nil {
		return fmt.Errorf("imap dial: %w", err)
	}

	if err := conn.Login(c.email, c.password).Wait(); err != nil {
		conn.Close()
		return fmt.Errorf("imap login: %w", err)
	}

	c.conn = conn
	return nil
}

// Close closes the IMAP connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Conn returns the underlying go-imap client for direct use.
func (c *Client) Conn() *goimapclient.Client {
	return c.conn
}
