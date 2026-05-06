//go:build !tinygo

package http

import nethttp "net/http"

func cloneTransport(tr *nethttp.Transport) *nethttp.Transport {
	return tr.Clone()
}

func configureTransport(tr *nethttp.Transport, opts Options) {
	if opts.HTTPProxy != nil {
		tr.Proxy = opts.HTTPProxy
	}

	if opts.TLS != nil {
		tr.TLSClientConfig = opts.TLS
	}
}
