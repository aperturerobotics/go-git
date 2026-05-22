//go:build goscript

// Package ssh implements the SSH transport for the new transport API.
package ssh

import (
	"context"
	"errors"

	"github.com/go-git/go-git/v6/plumbing/transport"
)

// Options configures the SSH transport.
type Options struct {
	// ClientConfig provides SSH client configuration for each request.
	ClientConfig any

	// DialContext is the function used to establish TCP connections.
	DialContext transport.DialContextFunc
}

// Transport implements the ssh:// transport protocol.
type Transport struct{}

// ErrUnsupportedTransport is returned when SSH transport execution is attempted
// in a GoScript build.
var ErrUnsupportedTransport = errors.New("ssh transport is not supported in goscript")

// NewTransport creates an SSH transport with the given options.
func NewTransport(Options) *Transport {
	return &Transport{}
}

// Connect implements transport.Connector.
func (t *Transport) Connect(context.Context, *transport.Request) (transport.Conn, error) {
	return nil, ErrUnsupportedTransport
}
