//go:build goscript

package git

// Default supported transports for GoScript builds.
import (
	_ "github.com/go-git/go-git/v6/plumbing/transport/file" // file transport
	_ "github.com/go-git/go-git/v6/plumbing/transport/git"  // git transport
)
