package warplib

import "net/http"

type Headers []Header

func (h Headers) Set(header http.Header) {
	for _, x := range h {
		x.Set(header)
	}
}

func (h Headers) Add(header http.Header) {
	for _, x := range h {
		x.Add(header)
	}
}

type Header struct {
	key, value string
}

func (h *Header) Set(header http.Header) {
	header.Set(h.key, h.value)
}

func (h *Header) Add(header http.Header) {
	header.Add(h.key, h.value)
}
