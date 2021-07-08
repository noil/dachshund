package dachshund

import "errors"

var (
	ErrDoPanic                      = errors.New("dachshund: unknown error.")
	ErrSendOnClosedChannelPanic     = errors.New("dachshund: send on closed channel.")
	ErrReleaseBufferedPool          = errors.New("dachshund: can't release pool, try it latter.")
	ErrReloadBufferedPoolInProgress = errors.New("dachshund: can't reload pool, try it latter.")
)
