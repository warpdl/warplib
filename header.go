package warplib

import "net/http"

type Header struct {
	key, value string
}

func (h *Header) Set(header http.Header) {
	header.Set(h.key, h.value)
}

func (h *Header) Add(header http.Header) {
	header.Add(h.key, h.value)
}
