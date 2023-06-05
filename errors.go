package warplib

import "errors"

var (
	ErrContentLengthInvalid        = errors.New("content length is invalid")
	ErrContentLengthNotImplemented = errors.New("unknown size downloads not implemented yet")
	ErrNotSupported                = errors.New("file you're trying to download is not supported yet")

	ErrDownloadNotFound = errors.New("Item you are trying to download is not found")
)
