//go:build !js

package zlib

import "reflect"

// isNil reports whether v is nil, including an interface that wraps a typed nil
// pointer, channel, func, map, or slice. The reflect-based typed-nil check is
// native-only: the browser build (js) keeps reflect out of the dependency graph
// and only receives concrete providers, which never carry a typed nil.
func isNil(v any) bool {
	if v == nil {
		return true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
