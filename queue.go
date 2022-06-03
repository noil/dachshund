package dachshund

import (
	"log"
	"sync"
)

const defaultTube = "default"

// Tuber interface for async jobs.
type Tuber interface {
	AsyncRun() func()
}

// Queue helps to delimit the code logically. Each tube has its own pool
type Queue struct {
	log   log.Logger
	tubes map[string]*Pool
	mu    sync.RWMutex
}

// NewQueue creates a new Queue
func NewQueue(count int, log log.Logger) *Queue {
	return &Queue{tubes: map[string]*Pool{
		defaultTube: NewPool(count, log),
	}}
}

// Add adds a new one
func (q *Queue) AddTube(tube string, size int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	_, ok := q.tubes[tube]
	if ok {
		return ErrTubeAlreadyExist
	}
	q.tubes[tube] = NewPool(size, q.log)
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
