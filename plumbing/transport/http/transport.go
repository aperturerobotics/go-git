//go:build !tinygo

package http

import nethttp "net/http"

func cloneTransport(tr *nethttp.Transport) *nethttp.Transport {
	return tr.Clone()
}
