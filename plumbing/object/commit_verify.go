//go:build !tinygo && !goscript

package object

import (
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"

	"github.com/go-git/go-git/v6/plumbing"
)

// Verify performs PGP verification of the commit with a provided armored
// keyring and returns openpgp.Entity associated with verifying key on success.
func (c *Commit) Verify(armoredKeyRing string) (*openpgp.Entity, error) {
	if countSignatureBlocks([]byte(c.Signature)) > 1 {
		return nil, ErrMultipleSignatures
	}

	keyRingReader := strings.NewReader(armoredKeyRing)
	keyring, err := openpgp.ReadArmoredKeyRing(keyRingReader)
	if err != nil {
		return nil, err
	}

	// Extract signature.
	signature := strings.NewReader(c.Signature)

	encoded := &plumbing.MemoryObject{}
	// Encode commit components, excluding signature and get a reader object.
	if err := c.EncodeWithoutSignature(encoded); err != nil {
		return nil, err
	}
	er, err := encoded.Reader()
	if err != nil {
		return nil, err
	}

	return openpgp.CheckArmoredDetachedSignature(keyring, er, signature, nil)
}
