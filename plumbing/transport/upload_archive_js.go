//go:build js

package transport

import (
	"context"
	"fmt"
	"io"

	"github.com/go-git/go-git/v6/storage"
)

// UploadArchiveRequest configures the server-side upload-archive service.
type UploadArchiveRequest struct{}

// UploadArchive reports that the git-upload-archive service is unavailable.
//
// Serving an archive streams a tar via archive/tar, which imports reflect. The
// browser GoScript closure (GOOS=js) never serves archives, so this build omits
// the archive backend to keep reflect out of the browser dependency graph and
// returns ErrCommandUnsupported for any upload-archive request.
func UploadArchive(_ context.Context, _ storage.Storer, _ io.ReadCloser, _ io.WriteCloser, _ *UploadArchiveRequest) error {
	return fmt.Errorf("%w: %s", ErrCommandUnsupported, UploadArchiveService)
}
