package warplib

import "log"

type (
	ErrorHandlerFunc            func(err error)
	SpawnPartHandlerFunc        func(hash string, ioff, foff int64)
	RespawnPartHandlerFunc      func(hash string, ioffNew, foffNew int64)
	ProgressHandlerFunc         func(hash string, nread int)
	DownloadCompleteHandlerFunc func(hash string, tread int64)
)

type Handlers struct {
	SpawnPartHandler        SpawnPartHandlerFunc
	RespawnPartHandler      RespawnPartHandlerFunc
	ProgressHandler         ProgressHandlerFunc
	ErrorHandler            ErrorHandlerFunc
	DownloadCompleteHandler DownloadCompleteHandlerFunc
}

func (h *Handlers) setDefault() {
	if h.SpawnPartHandler == nil {
		h.SpawnPartHandler = func(hash string, ioff, foff int64) {}
	}
	if h.RespawnPartHandler == nil {
		h.RespawnPartHandler = func(hash string, ioffNew, foffNew int64) {}
	}
	if h.ProgressHandler == nil {
		h.ProgressHandler = func(hash string, nread int) {}
	}
	if h.DownloadCompleteHandler == nil {
		h.DownloadCompleteHandler = func(hash string, tread int64) {}
	}
	if h.ErrorHandler == nil {
		h.ErrorHandler = func(err error) {
			log.Println(err)
		}
	}
}
