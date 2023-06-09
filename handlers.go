package warplib

import "log"

type (
	ErrorHandlerFunc            func(hash string, err error)
	SpawnPartHandlerFunc        func(hash string, ioff, foff int64)
	RespawnPartHandlerFunc      func(hash string, partIoff, ioffNew, foffNew int64)
	DownloadProgressHandlerFunc func(hash string, nread int)
	ResumeProgressHandlerFunc   func(nread int)
	DownloadCompleteHandlerFunc func(hash string, tread int64)
	CompileStartHandlerFunc     func()
	CompileProgressHandlerFunc  func(nread int)
	CompileCompleteHandlerFunc  func()
)

type Handlers struct {
	SpawnPartHandler        SpawnPartHandlerFunc
	RespawnPartHandler      RespawnPartHandlerFunc
	DownloadProgressHandler DownloadProgressHandlerFunc
	ResumeProgressHandler   ResumeProgressHandlerFunc
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
		h.RespawnPartHandler = func(hash string, partIoff, ioffNew, foffNew int64) {}
	}
	if h.DownloadProgressHandler == nil {
		h.DownloadProgressHandler = func(hash string, nread int) {}
	}
	if h.ResumeProgressHandler == nil {
		h.ResumeProgressHandler = func(nread int) {}
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
