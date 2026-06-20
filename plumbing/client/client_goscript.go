//go:build goscript

// Package client provides a convenience Client that resolves URL schemes
// to transport implementations and provides Handshake/Connect methods.
package client

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/plumbing/transport/file"
	xgit "github.com/go-git/go-git/v6/plumbing/transport/git"
)

// SSHAuth is implemented by SSH authentication types whose ClientConfig
// method can be used to produce an *ssh.ClientConfig for each request.
type SSHAuth interface{}

// Option configures a Client.
type Option func(*options)

type options struct {
	git  xgit.Options
	file file.Options

	schemes map[string]transport.Transport
}

// WithSSHAuth accepts SSH authentication for API parity with the native
// client. The GoScript build omits the ssh transport to keep reflect (pulled
// by golang.org/x/crypto/ssh) out of the browser dependency graph, so the
// browser never serves ssh:// URLs and this option has no effect.
func WithSSHAuth(a SSHAuth) Option {
	return func(o *options) {
	}
}

// WithDialer sets a custom dialer for the Git TCP transport.
func WithDialer(fn transport.DialContextFunc) Option {
	return func(o *options) {
		o.git.DialContext = fn
	}
}

// WithLoader sets the storage loader for the file transport.
func WithLoader(l transport.Loader) Option {
	return func(o *options) {
		o.file.Loader = l
	}
}

// WithTransport registers a custom transport for the given URL scheme.
// This overrides any built-in transport for that scheme.
func WithTransport(scheme string, tr transport.Transport) Option {
	return func(o *options) {
		if scheme == "" || tr == nil {
			return
		}
		if o.schemes == nil {
			o.schemes = make(map[string]transport.Transport)
		}
		o.schemes[scheme] = tr
	}
}

// Client resolves URL schemes to transport implementations.
type Client struct {
	opts options
}

// New creates a Client with GoScript-compatible built-in transports.
func New(opts ...Option) *Client {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return &Client{opts: o}
}

// Handshake resolves the transport for the request URL scheme and performs
// a pack protocol handshake.
func (c *Client) Handshake(ctx context.Context, req *transport.Request) (transport.Session, error) {
	tr, err := c.resolve(req)
	if err != nil {
		return nil, err
	}
	return tr.Handshake(ctx, req)
}

// Connect resolves the transport for the request URL scheme and opens a
// raw full-duplex connection.
func (c *Client) Connect(ctx context.Context, req *transport.Request) (transport.Conn, error) {
	tr, err := c.resolve(req)
	if err != nil {
		return nil, err
	}
	conn, ok := tr.(transport.Connector)
	if !ok {
		return nil, fmt.Errorf("transport for %s does not support Connect: %w", req.URL.Scheme, transport.ErrConnectUnsupported)
	}
	return conn.Connect(ctx, req)
}

// Transport returns the resolved transport for the given URL scheme.
func (c *Client) Transport(scheme string) (transport.Transport, error) {
	if c.opts.schemes != nil {
		if tr, ok := c.opts.schemes[scheme]; ok {
			return tr, nil
		}
	}
	return c.builtin(scheme)
}

// Close releases resources held by the client.
func (c *Client) Close() error {
	return nil
}

func (c *Client) resolve(req *transport.Request) (transport.Transport, error) {
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("transport: nil request or URL")
	}
	return c.Transport(req.URL.Scheme)
}

func (c *Client) builtin(scheme string) (transport.Transport, error) {
	switch scheme {
	case "file":
		return file.NewTransport(c.opts.file), nil
	case "git":
		return xgit.NewTransport(c.opts.git), nil
	default:
		return nil, fmt.Errorf("transport: unsupported scheme %q", scheme)
	}
}
