package dachshund

import "errors"

var (
	ErrDoTaskPanic                  = errors.New("unknown error.")
	ErrSendOnClosedChannelPanic     = errors.New("send on closed channel.")
	ErrReleaseBufferedPool          = errors.New("can't release pool, try it latter.")
	ErrReloadBufferedPoolInProgress = errors.New("can't reload pool, try it latter.")
)
