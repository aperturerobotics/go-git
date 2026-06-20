//go:build !js

package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/go-git/go-git/v6/internal/archive"
	"github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/protocol/packp/sideband"
	"github.com/go-git/go-git/v6/utils/ioutil"
)

// git-archive support pulls archive/tar, which imports reflect. The browser
// GoScript closure (GOOS=js) never creates archives, so this surface is built
// only for native targets to keep reflect out of the browser dependency graph.

// ArchiveOptions stores the options for the Archive operation.
type ArchiveOptions struct {
	// Format is the archive format.
	// Archive supports tar, tar.gz, tgz, and zip locally.
	// ArchiveRemote passes the format through to the remote server, so
	// supported values are server-dependent.
	// Defaults to tar.
	Format string
	// Prefix is an optional prefix to prepend to each pathname.
	Prefix string
	// Treeish is the tree-ish object to archive (commit, tag, or tree).
	Treeish string
	// Paths is an optional list of paths to include in the archive.
	// If empty, all paths are included.
	Paths []string
	// ClientOptions are options for the transport client (used by ArchiveRemote).
	ClientOptions []client.Option
	// Progress receives human-readable status from the remote server.
	// Only used by ArchiveRemote, ignored by Archive.
	Progress sideband.Progress
}

// Validate validates the ArchiveOptions.
func (o *ArchiveOptions) Validate() error {
	if o.Treeish == "" {
		return errors.New("tree-ish is required")
	}

	if o.Prefix != "" && archive.HasInvalidPrefix(o.Prefix) {
		return fmt.Errorf("%w: %s", archive.ErrInvalidPrefix, o.Prefix)
	}

	return nil
}

// Archive creates an archive from the local repository.
// It returns an io.ReadCloser that yields the archive data.
// The caller must close the returned ReadCloser.
func (r *Repository) Archive(o *ArchiveOptions) (io.ReadCloser, error) {
	return r.ArchiveContext(context.Background(), o)
}

// ArchiveContext creates an archive from the local repository.
// The provided Context can be used to cancel the operation.
func (r *Repository) ArchiveContext(ctx context.Context, o *ArchiveOptions) (io.ReadCloser, error) {
	if o == nil {
		o = &ArchiveOptions{}
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}

	format := o.Format
	if format == "" {
		format = "tar"
	}

	if !slices.Contains(archive.SupportedFormats(), format) {
		return nil, fmt.Errorf("%w: %s", archive.ErrUnsupportedFormat, format)
	}

	// Always allow unreachable refs for local archives.
	tree, commitHash, commitTime, err := archive.ResolveTreeish(r.Storer, o.Treeish, true)
	if err != nil {
		return nil, err
	}

	prefix := o.Prefix
	paths := slices.Clone(o.Paths)

	pr, pw := io.Pipe()
	cw := ioutil.NewContextWriter(ctx, pw)
	go func() {
		err := archive.WriteArchive(r.Storer, cw, tree, commitHash, commitTime, format, prefix, paths)
		_ = pw.CloseWithError(err)
	}()

	return pr, nil
}
