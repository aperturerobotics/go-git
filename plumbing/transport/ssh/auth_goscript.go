//go:build goscript

package ssh

import "errors"

// PublicKeys implements SSH public key authentication using the given key pair.
type PublicKeys struct {
	User   string
	Signer any
}

// ErrUnsupportedAuth is returned when SSH auth helpers are used in a GoScript
// build.
var ErrUnsupportedAuth = errors.New("ssh auth is not supported in goscript")

// NewPublicKeys returns a PublicKeys from a PEM encoded private key.
func NewPublicKeys(user string, _ []byte, _ string) (*PublicKeys, error) {
	return &PublicKeys{User: user}, ErrUnsupportedAuth
}

// NewPublicKeysFromFile returns a PublicKeys from a file containing a PEM
// encoded private key.
func NewPublicKeysFromFile(user, _, _ string) (*PublicKeys, error) {
	return &PublicKeys{User: user}, ErrUnsupportedAuth
}
