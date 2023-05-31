package warplib

import "log"

type (
	ErrorHandlerFunc            func(hash string, err error)
	SpawnPartHandlerFunc        func(hash string, ioff, foff int64)
	RespawnPartHandlerFunc      func(hash string, ioffNew, foffNew int64)
	ProgressHandlerFunc         func(hash string, nread int)
	DownloadCompleteHandlerFunc func(hash string, tread int64)
	CompileStartHandlerFunc     func()
	CompileProgressHandlerFunc  func(nread int)
	CompileCompleteHandlerFunc  func()
)

type Handlers struct {
	SpawnPartHandler        SpawnPartHandlerFunc
	RespawnPartHandler      RespawnPartHandlerFunc
	ProgressHandler         ProgressHandlerFunc
	ErrorHandler            ErrorHandlerFunc
	DownloadCompleteHandler DownloadCompleteHandlerFunc
	CompileStartHandler     CompileStartHandlerFunc
	CompileProgressHandler  CompileProgressHandlerFunc
	CompileCompleteHandler  CompileCompleteHandlerFunc
}

func (h *Handlers) setDefault(l *log.Logger) {
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
	if h.CompileStartHandler == nil {
		h.CompileStartHandler = func() {}
	}
	if h.CompileProgressHandler == nil {
		h.CompileProgressHandler = func(nread int) {}
	}
	if h.CompileCompleteHandler == nil {
		h.CompileCompleteHandler = func() {}
	}
	if h.ErrorHandler == nil {
		h.ErrorHandler = func(hash string, err error) {
			l.Printf("%s: %s\n", hash, err.Error())
		}
	}
}
