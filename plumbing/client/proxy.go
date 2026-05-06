//go:build !tinygo

package client

import (
	"net/http"
	"net/url"
)

func httpProxyURL(u *url.URL) func(*http.Request) (*url.URL, error) {
	return http.ProxyURL(u)
}

func httpProxyFromEnvironment() func(*http.Request) (*url.URL, error) {
	return http.ProxyFromEnvironment
}
