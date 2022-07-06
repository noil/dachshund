package dachshund

import (
	"sync"
	"sync/atomic"
)

type Tube struct {
	pool            *Pool
	job             chan func()
	countActiveJobs int64
	stop            chan struct{}
	mu              sync.RWMutex
	closed          bool
}

func initTube(size int64, opts ...Option) *Tube {
	t := &Tube{
		pool: NewPool(size, opts...),
		job:  make(chan func()),
		stop: make(chan struct{}),
	}
	go t.launch()

	return t
}

func (t *Tube) launch() {
Loop:
	for {
		select {
		case <-t.stop:
			break Loop
		case job := <-t.job:
			t.pool.Do(job)
			t.dec()
		}
	}
}

func (t *Tube) inc() {
	atomic.AddInt64(&t.countActiveJobs, 1)
}

func (t *Tube) dec() {
	atomic.AddInt64(&t.countActiveJobs, -1)
}

func (t *Tube) Terminate() {
	t.closed = true

	for atomic.LoadInt64(&t.countActiveJobs) > 0 {
	}
	t.stop <- struct{}{}
}
