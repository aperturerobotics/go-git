//go:build js

package revfile

import "io"

// isNilWriter reports whether w is a nil io.Writer. The browser never passes a
// typed-nil writer, so the reflect-based typed-nil check from the native build
// is omitted here to keep reflect out of the browser dependency graph.
func isNilWriter(w io.Writer) bool {
	return w == nil
}
