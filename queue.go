package dachshund

import (
	"context"
)

const defaultTube = "default"

// DefaultTube returns default tube
func DefaultTube() string {
	return defaultTube
}

// Tuber interface for async jobs.
type Tuber interface {
	AsyncRun() func()
}

// Queue helps to delimit the code logically. Each tube has its own pool
type Queue struct {
	opts  []Option
	tubes map[string]*Tube
}

// NewQueue creates a new Queue
func NewQueue(size int64, opts ...Option) *Queue {
	return NewQueueWithContext(context.Background(), size, opts...)
}

// NewQueue creates a new Queue
func NewQueueWithContext(ctx context.Context, size int64, opts ...Option) *Queue {
	return &Queue{
		tubes: map[string]*Tube{
			defaultTube: initTube(ctx, size, opts...),
		},
		opts: opts,
	}
}

// Terminate deletes all tubes
func (q *Queue) Terminate() error {
	for key, t := range q.tubes {
		if !t.isClosed() {
			t.stop <- struct{}{}
		}
		delete(q.tubes, key)
	}
	return nil
}

// AddTube adds a new one
func (q *Queue) AddTube(tube string, size int64) error {
	return q.AddTubeWithContext(context.Background(), tube, size)
}

// AddTubeWithContext adds a new one with context
func (q *Queue) AddTubeWithContext(ctx context.Context, tube string, size int64) error {
	_, ok := q.tubes[tube]
	if ok {
		return ErrTubeAlreadyExist
	}
	q.tubes[tube] = initTube(ctx, size, q.opts...)
	return nil
}

// Terminate remove a tube
func (q *Queue) TerminateTube(tube string) error {
	t, ok := q.tubes[tube]
	if !ok {
		return ErrTubeNotFound
	}
	t.shutdowning()
	delete(q.tubes, tube)
	return nil
}

// PushFunc runs a func in a specific tube, or returns error if tune not found
func (q *Queue) PushFunc(tube string, job func()) error {
	t, ok := q.tubes[tube]
	if !ok {
		return ErrTubeNotFound
	}
	if t.isClosed() {
		return ErrTubeClosed
	}
	go func() {
		t.wg.Add(1)
		t.job <- job
	}()
	return nil
}

// Push runs a struct task called AsyncRun() in a specific tube, or returns error if tune not found
func (q *Queue) Push(tube string, job Tuber) error {
	t, ok := q.tubes[tube]
	if !ok {
		return ErrTubeNotFound
	}
	if t.isClosed() {
		return ErrTubeClosed
	}
	go func() {
		t.wg.Add(1)
		t.job <- job.AsyncRun()
	}()
	return nil
}
