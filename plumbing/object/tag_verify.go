//go:build !tinygo

package object

import (
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"

	"github.com/go-git/go-git/v6/plumbing"
)

// Verify performs PGP verification of the tag with a provided armored
// keyring and returns openpgp.Entity associated with verifying key on
// success.
func (t *Tag) Verify(armoredKeyRing string) (*openpgp.Entity, error) {
	keyRingReader := strings.NewReader(armoredKeyRing)
	keyring, err := openpgp.ReadArmoredKeyRing(keyRingReader)
	if err != nil {
		return nil, err
	}

	// Extract signature.
	signature := strings.NewReader(t.Signature)

	encoded := &plumbing.MemoryObject{}
	// Encode tag components, excluding signature and get a reader object.
	if err := t.EncodeWithoutSignature(encoded); err != nil {
		return nil, err
	}
	er, err := encoded.Reader()
	if err != nil {
		return nil, err
	}

	return openpgp.CheckArmoredDetachedSignature(keyring, er, signature, nil)
}
