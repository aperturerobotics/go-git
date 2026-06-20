//go:build !js

package revfile

import (
	"io"
	"reflect"
)

// isNilWriter reports whether w is a nil io.Writer, including an interface that
// wraps a typed nil pointer. The reflect-based typed-nil check is native-only:
// the browser build (js) keeps reflect out of the dependency graph and only
// internal callers reach this path, which never pass a typed-nil writer.
func isNilWriter(w io.Writer) bool {
	if w == nil {
		return true
	}
	v := reflect.ValueOf(w)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	}
	return false
}
