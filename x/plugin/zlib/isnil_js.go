//go:build js

package zlib

// isNil reports whether v is a nil interface. The browser receives only
// concrete zlib providers, so the reflect-based typed-nil check from the native
// build is omitted here to keep reflect out of the browser dependency graph.
func isNil(v any) bool {
	return v == nil
}
