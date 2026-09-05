package packfile

import (
	"bytes"
	"testing"

	"github.com/go-git/go-git/v6/plumbing"
)

// TestParserReleasesConsumedDeltaBases bounds retained contents during a delta chain.
func TestParserReleasesConsumedDeltaBases(t *testing.T) {
	for _, kind := range []plumbing.ObjectType{plumbing.REFDeltaObject, plumbing.OFSDeltaObject} {
		t.Run(kind.String(), func(t *testing.T) {
			// Build a chain and a late sibling that still needs the original base.
			content := []byte{0, 0}
			base := testObjectHash(plumbing.BlobObject, content)
			parent := base
			objects := []testPackObject{{typ: plumbing.BlobObject, content: content}}
			for i := 1; i <= 40; i++ {
				content = []byte{byte(i), 0}
				encoded, offsets := buildTestPack(t, objects...)
				objects = append(objects, testPackObject{
					typ:                 kind,
					offsetDeltaDistance: int64(len(encoded)-20) - offsets[len(offsets)-1],
					content:             buildDelta(2, 2, insertOp(content)),
					reference:           parent,
				})
				parent = testObjectHash(plumbing.BlobObject, content)
			}
			encoded, offsets := buildTestPack(t, objects...)
			objects = append(objects, testPackObject{
				typ:                 kind,
				offsetDeltaDistance: int64(len(encoded)-20) - offsets[0],
				content:             buildDelta(2, 2, insertOp([]byte{99, 0})),
				reference:           base,
			})
			data, _ := buildTestPack(t, objects...)

			// A non-seekable source cannot recover an incorrectly released base.
			observer := &retentionObserver{}
			parser := NewParser(bytes.NewBuffer(data), WithScannerObservers(observer))
			observer.parser = parser
			if _, err := parser.Parse(); err != nil {
				t.Fatal(err)
			}

			// Only the shared original base, current parent, and current child stay live.
			if observer.peak > 3 {
				t.Fatalf("retained %d inflated objects, want at most 3", observer.peak)
			}
			if observer.objects != len(objects) {
				t.Fatalf("parsed %d objects, want %d", observer.objects, len(objects))
			}
			for _, object := range parser.cache.oi {
				if object.content != nil {
					t.Fatal("Parse returned while cached content remained live")
				}
			}
		})
	}
}

// retentionObserver measures inflated content retained by a sequential parse.
type retentionObserver struct {
	// parser supplies the live cache at observer boundaries.
	parser *Parser
	// peak is the maximum number of cached inflated objects.
	peak int
	// objects counts parsed object notifications.
	objects int
}

// OnHeader accepts the declared object count.
func (o *retentionObserver) OnHeader(uint32) error { return nil }

// OnInflatedObjectHeader accepts the parsed object header.
func (o *retentionObserver) OnInflatedObjectHeader(plumbing.ObjectType, int64, int64) error {
	return nil
}

// OnInflatedObjectContent samples retained buffers while parsing still owns them.
func (o *retentionObserver) OnInflatedObjectContent(plumbing.Hash, int64, uint32, []byte) error {
	count := 0
	for _, object := range o.parser.cache.oi {
		if object.content != nil {
			count++
		}
	}
	o.peak = max(o.peak, count)
	o.objects++
	return nil
}

// OnFooter accepts the verified pack checksum.
func (o *retentionObserver) OnFooter(plumbing.Hash) error { return nil }

// _ is a type assertion
var _ Observer = (*retentionObserver)(nil)
