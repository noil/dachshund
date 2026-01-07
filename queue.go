package dachshund

import (
	"context"
	"sync"
)

// Tuber interface for async jobs.
type Tuber interface {
	AsyncRun() func()
}

// Queue helps to delimit the code logically. Each tube has its own pool
type Queue struct {
	opts   []Option
	tubes  map[string]*Tube
	closed bool
	mu     sync.RWMutex
}

// NewQueue creates a new Queue
func NewQueue(opts ...Option) *Queue {
	return &Queue{
		tubes: map[string]*Tube{},
		opts:  opts,
	}
}

// NewQueueWithContext creates a new Queue
func NewQueueWithContext(ctx context.Context, opts ...Option) *Queue {
	queue := &Queue{
		tubes: map[string]*Tube{},
		opts:  opts,
	}
	go func() {
		<-ctx.Done()
		queue.Terminate()
	}()
	return queue
}

// Terminate deletes all tubes
func (q *Queue) Terminate() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	q.closed = true
	for _, t := range q.tubes {
		t.Terminate()
		close(t.job)
		close(t.stop)
		t.pool.Release()
	}
	q.tubes = map[string]*Tube{}
}

// AddTube adds a new one
func (q *Queue) AddTube(tube string, size int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}
	_, ok := q.tubes[tube]
	if ok {
		return ErrTubeAlreadyExist
	}
	q.tubes[tube] = initTube(size, q.opts...)
	return nil
}

// TerminateTube remove a tube
func (q *Queue) TerminateTube(tube string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	t, ok := q.tubes[tube]
	if !ok {
		return
	}
	delete(q.tubes, tube)
	t.Terminate()
	close(t.job)
	close(t.stop)
	t.pool.Release()
}

// PushFunc runs a func in a specific tube, or returns error if tune not found
func (q *Queue) PushFunc(tube string, job func()) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return ErrTubeClosed
	}
	t, ok := q.tubes[tube]
	if !ok {
		return ErrTubeNotFound
	}
	if t.closed {
		return ErrTubeClosed
	}
	t.inc()
	go func() {
		defer func() {
			if recover() != nil {
				t.dec()
			}
		}()
		t.job <- job
	}()
	return nil
}

// Push runs a struct task called AsyncRun() in a specific tube, or returns error if tune not found
func (q *Queue) Push(tube string, job Tuber) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.closed {
		return ErrTubeClosed
	}
	t, ok := q.tubes[tube]
	if !ok {
		return ErrTubeNotFound
	}
	if t.closed {
		return ErrTubeClosed
	}
	t.inc()
	go func() {
		defer func() {
			if recover() != nil {
				t.dec()
			}
		}()
		t.job <- job.AsyncRun()
	}()
	return nil
}
