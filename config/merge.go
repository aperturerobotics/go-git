//go:build !tinygo

package config

import "reflect"

func merge(dst, src any) {
	tv := reflect.ValueOf(dst).Elem()
	sv := reflect.ValueOf(src).Elem()
	tt := tv.Type()

	for i := 0; i < tv.NumField(); i++ {
		// Raw holds a *format.Config whose Sections field is a slice. The
		// generic default case below would replace dst.Sections with
		// src.Sections wholesale, dropping sections that exist only in the
		// base config (e.g. [extensions]). Merge() rebuilds Raw from the
		// merged struct state after all sources have been processed, so we
		// skip it here.
		if tt.Field(i).Name == "Raw" {
			continue
		}

		df := tv.Field(i)
		sf := sv.Field(i)

		if !df.CanSet() || sf.IsZero() {
			continue
		}

		switch df.Kind() {
		case reflect.Struct:
			// Handle nested fields which are based off structs.
			merge(df.Addr().Interface(), sf.Addr().Interface())

		case reflect.Pointer:
			if sf.IsNil() {
				continue
			}
			if df.IsNil() {
				df.Set(reflect.New(df.Type().Elem()))
			}
			// Same as per reflect.Struct, but for a struct pointer.
			if df.Elem().Kind() == reflect.Struct {
				merge(df.Interface(), sf.Interface())
			} else {
				df.Set(sf)
			}

		case reflect.Map:
			// An empty (but non-nil) src map must not overwrite dst entries.
			// Only copy individual entries from src so that dst keys not
			// present in src are preserved and src entries override same-key
			// dst entries.
			if sf.Len() == 0 {
				continue
			}
			if df.IsNil() {
				df.Set(reflect.MakeMap(df.Type()))
			}
			for _, key := range sf.MapKeys() {
				df.SetMapIndex(key, sf.MapIndex(key))
			}

		default:
			df.Set(sf)
		}
	}
}
