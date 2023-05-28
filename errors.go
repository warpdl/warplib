package warplib

import "errors"

var (
	ErrContentLengthInvalid        = errors.New("contentLengthLnvalid")
	ErrContentLengthNotImplemented = errors.New("unknown size downloads not implemented yet")
	ErrNotSupported                = errors.New("file you're trying to download is not supported yet")
)
