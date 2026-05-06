//go:build tinygo

package config

import "reflect"

func merge(dst, src any) {
	tv := reflect.ValueOf(dst).Elem()
	sv := reflect.ValueOf(src).Elem()
	tt := tv.Type()

	for i := 0; i < tv.NumField(); i++ {
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
			merge(df.Addr().Interface(), sf.Addr().Interface())

		case reflect.Pointer:
			if sf.IsNil() {
				continue
			}
			if df.IsNil() {
				df.Set(reflect.New(df.Type().Elem()))
			}
			if df.Elem().Kind() == reflect.Struct {
				merge(df.Interface(), sf.Interface())
			} else {
				df.Set(sf)
			}

		case reflect.Map:
			if sf.Len() != 0 {
				df.Set(sf)
			}

		default:
			df.Set(sf)
		}
	}
}
