package dachshund

import (
	"context"
	"sync"
	"sync/atomic"
)

type Tube struct {
	pool   *Pool
	job    chan func()
	stop   chan struct{}
	wg     sync.WaitGroup
	closed int32
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
	return atomic.LoadInt32(&t.closed) == 1
}

func (t *Tube) Close() bool {
	return atomic.CompareAndSwapInt32(&t.closed, 0, 1)
}
