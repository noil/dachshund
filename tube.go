package dachshund

import (
	"context"
	"sync"
)

type Tube struct {
	pool   *Pool
	job    chan func()
	stop   chan struct{}
	wg     sync.WaitGroup
	mu     sync.RWMutex
	closed bool
}

func initTube(ctx context.Context, size int64, opts ...Option) *Tube {
	t := &Tube{
		pool: NewPool(size, opts...),
		job:  make(chan func()),
		stop: make(chan struct{}),
	}
	go t.launch(ctx)

	return t
}

func (t *Tube) launch(ctx context.Context) {
Loop:
	for {
		select {
		case <-t.stop:
		case <-ctx.Done():
			break Loop
		case job := <-t.job:
			t.pool.Do(job)
			t.wg.Done()
		}
	}
	t.shutdowning()
}

func (t *Tube) shutdowning() {
	go func() {
		for job := range t.job {
			t.pool.Do(job)
			t.wg.Done()
		}
	}()
	t.wg.Wait()
	close(t.job)
	close(t.stop)
	t.pool.Release()
}

func (t *Tube) IsClosed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.closed
}

func (t *Tube) TryClosing() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.closed = true
		return true
	}
	return false
}
