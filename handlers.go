package warplib

type (
	ErrorHandlerFunc       func(err error)
	SpawnPartHandlerFunc   func(hash string, ioff, foff int64)
	RespawnPartHandlerFunc func(hash string, nread, ioffNew, foffNew int64)
	ProgressHandlerFunc    func(hash string, nread int)
)

type Handlers struct {
	SpawnPartHandler   SpawnPartHandlerFunc
	RespawnPartHandler RespawnPartHandlerFunc
	ProgressHandler    ProgressHandlerFunc
	ErrorHandler       ErrorHandlerFunc
}
