//go:build tinygo

package client

import (
	"net/http"
	"net/url"
)

func httpProxyURL(*url.URL) func(*http.Request) (*url.URL, error) {
	return nil
}

func httpProxyFromEnvironment() func(*http.Request) (*url.URL, error) {
	return nil
}
