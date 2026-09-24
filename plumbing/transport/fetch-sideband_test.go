package transport

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/format/packfile"
	"github.com/go-git/go-git/v6/plumbing/protocol/capability"
	"github.com/go-git/go-git/v6/plumbing/protocol/packp/sideband"
	"github.com/go-git/go-git/v6/storage/memory"
)

// TestFetchPackSidebandWithoutProgress verifies that progress output is optional,
// but sideband decoding is required to deliver a pack rather than pkt-lines.
func TestFetchPackSidebandWithoutProgress(t *testing.T) {
	source := memory.NewStorage()
	obj := source.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	const content = "sideband fetch without progress"
	obj.SetSize(int64(len(content)))
	writer, err := obj.Writer()
	require.NoError(t, err)
	_, err = io.WriteString(writer, content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	want, err := source.SetEncodedObject(obj)
	require.NoError(t, err)
	var pack bytes.Buffer
	encoder := packfile.NewEncoder(&pack, source, false)
	_, err = encoder.Encode([]plumbing.Hash{want}, 0)
	require.NoError(t, err)

	for _, tc := range []struct {
		name       string
		mode       sideband.Type
		capability capability.Capability
	}{
		{"sideband-64k", sideband.Sideband64k, capability.Sideband64k},
		{"sideband", sideband.Sideband, capability.Sideband},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var wire bytes.Buffer
			mux := sideband.NewMuxer(tc.mode, &wire)
			_, err := mux.WriteChannel(sideband.ProgressMessage, []byte("receiving objects\n"))
			require.NoError(t, err)
			_, err = mux.Write(pack.Bytes())
			require.NoError(t, err)
			require.Equal(t, byte('0'), wire.Bytes()[0], "pack input must begin with pkt-line framing")

			caps := capability.List{}
			caps.Add(tc.capability)
			var progress bytes.Buffer
			withProgress := memory.NewStorage()
			err = FetchPack(context.Background(), withProgress, caps,
				io.NopCloser(bytes.NewReader(wire.Bytes())), nil, &FetchRequest{Progress: &progress})
			require.NoError(t, err)
			require.Equal(t, "receiving objects\n", progress.String())
			obj, err := withProgress.EncodedObject(plumbing.BlobObject, want)
			require.NoError(t, err)
			require.Equal(t, want, obj.Hash())

			withoutProgress := memory.NewStorage()
			err = FetchPack(context.Background(), withoutProgress, caps,
				io.NopCloser(bytes.NewReader(wire.Bytes())), nil, &FetchRequest{})
			require.NoError(t, err, "sideband frames must be decoded even without a progress writer")
			obj, err = withoutProgress.EncodedObject(plumbing.BlobObject, want)
			require.NoError(t, err)
			require.Equal(t, want, obj.Hash())
		})
	}
}
