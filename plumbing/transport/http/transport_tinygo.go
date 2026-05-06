//go:build tinygo

package http

import nethttp "net/http"

func cloneTransport(tr *nethttp.Transport) *nethttp.Transport {
	cloned := *tr
	return &cloned
}

func configureTransport(*nethttp.Transport, Options) {}
