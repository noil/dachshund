package dachshund

import "errors"

var (
	ErrDoPanic                  = errors.New("dachshund: unknown error.")
	ErrSendOnClosedChannelPanic = errors.New("dachshund: send on closed channel.")
	ErrTubeAlreadyExist         = errors.New("dachshund: tube already exists")
	ErrTubeNotFound             = errors.New("dachshund: tube not found")
	ErrTubeClosed               = errors.New("dachshund: tube closed")
	ErrQueueClosed              = errors.New("dachshund: queue closed")
)
