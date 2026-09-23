//go:build js

package git

// reusableRootFS uses the worktree filesystem on JS, where os.Root is absent.
func (w *Worktree) reusableRootFS() (*worktreeFilesystem, func()) {
	return w.filesystem, func() {}
}
