package dachshund

import (
	"context"
	"sync"
)

var mu sync.Mutex

// Tuber interface for async jobs.
type Tuber interface {
	AsyncRun() func()
}

// Queue helps to delimit the code logically. Each tube has its own pool
type Queue struct {
	opts   []Option
	tubes  map[string]*Tube
	closed bool
}

// NewQueue creates a new Queue
func NewQueue(opts ...Option) *Queue {
	return &Queue{
		tubes: map[string]*Tube{},
		opts:  opts,
	}
}

// NewQueue creates a new Queue
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
	mu.Lock()
	defer mu.Unlock()

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
	mu.Lock()
	defer mu.Unlock()

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
	mu.Lock()
	defer mu.Unlock()

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
	mu.Lock()
	defer mu.Unlock()

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
	go func() {
		t.inc()
		t.job <- job
	}()
	return nil
}

// Push runs a struct task called AsyncRun() in a specific tube, or returns error if tune not found
func (q *Queue) Push(tube string, job Tuber) error {
	mu.Lock()
	defer mu.Unlock()

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
	go func() {
		t.inc()
		t.job <- job.AsyncRun()
	}()
	return nil
}
