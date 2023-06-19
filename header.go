package warplib

import "net/http"

const (
	__USER_AGENT_KEY = "User-Agent"
)

type Headers []Header

func (h Headers) Get(key string) (index int, have bool) {
	for i, x := range h {
		if x.key != key {
			continue
		}
		index = i
		have = true
		break
	}
	return
}

func (h *Headers) Update(key, value string) {
	i, ok := h.Get(key)
	if ok {
		(*h)[i] = Header{key, value}
		return
	}
	*h = append(*h, Header{key, value})
}

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
