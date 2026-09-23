//go:build !js

package git

import (
	"os"

	"github.com/go-git/go-billy/v6/osfs"
)

// reusableRootFS returns a worktree filesystem to use for a single bulk
// checkout/reset. An OS-backed filesystem reuses one root for the operation.
func (w *Worktree) reusableRootFS() (*worktreeFilesystem, func()) {
	bos, ok := w.filesystem.Filesystem.(*osfs.BoundOS)
	if !ok {
		return w.filesystem, func() {}
	}
	root, err := os.OpenRoot(bos.Root())
	if err != nil {
		return w.filesystem, func() {}
	}
	rfs, err := osfs.FromRoot(root)
	if err != nil {
		_ = root.Close()
		return w.filesystem, func() {}
	}
	return newWorktreeFilesystem(rfs, w.filesystem.protectNTFS, w.filesystem.protectHFS),
		func() { _ = root.Close() }
}
