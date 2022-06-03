package dachshund

import (
	"context"
	"sync"
)

const defaultTube = "default"

// Tuber interface for async jobs.
type Tuber interface {
	AsyncRun() func()
}

// Queue helps to delimit the code logically. Each tube has its own pool
type Queue struct {
	opts  []Option
	tubes map[string]*Pool
	mu    sync.RWMutex
}

// NewQueue creates a new Queue
func NewQueue(size int64, opts ...Option) *Queue {
	return NewQueueWithContext(context.Background(), size, opts...)
}

// NewQueue creates a new Queue
func NewQueueWithContext(ctx context.Context, size int64, opts ...Option) *Queue {
	return &Queue{
		tubes: map[string]*Pool{
			defaultTube: NewPoolWithContext(ctx, size, opts...),
		},
		opts: opts,
	}
}

// Add adds a new one
func (q *Queue) AddTube(tube string, size int64) error {
	return q.AddTubeWithContext(context.Background(), tube, size)
}

func (q *Queue) AddTubeWithContext(ctx context.Context, tube string, size int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	_, ok := q.tubes[tube]
	if ok {
		return ErrTubeAlreadyExist
	}
	q.tubes[tube] = NewPoolWithContext(ctx, size, q.opts...)
	return nil
}

// PushFunc runs a func in a specific tube, or returns error if tune not found
func (q *Queue) PushFunc(tube string, job func()) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	pool, ok := q.tubes[tube]
	if !ok {
		return ErrTubeNotFound
	}
	pool.Do(job)
	return nil
}

// Push runs a struct task called AsyncRun() in a specific tube, or returns error if tune not found
func (q *Queue) Push(tube string, job Tuber) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	pool, ok := q.tubes[tube]
	if !ok {
		return ErrTubeNotFound
	}
	pool.Do(job.AsyncRun())
	return nil
}
