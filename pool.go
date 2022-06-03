package dachshund

import (
	"context"
	"sync/atomic"
	"time"
)

const (
	resize      = 1
	enlarge     = 2
	reduce      = 3
	closeWorker = 4
	closingPool = 5
	closedPool  = 6
)

type System int
type worker struct {
	jobChan      chan func()
	jobPoolChan  chan chan func()
	panicHandler func(any)
}

type Pool struct {
	jobChan             chan chan func()
	countWorkers        int64
	currentCountWorkers int64
	system              chan System
	terminateWorker     int32
	panicHandler        func(any)
}

// Option enriches default behavior
type Option func(*Pool)

// Function to handle panics recovered
func WithPanicHandler(handler func(any)) Option {
	return func(p *Pool) {
		p.panicHandler = handler
	}
}

// NewPool returns a Pool struct
func NewPool(size int64, opts ...Option) *Pool {
	return NewPoolWithContext(context.Background(), size, opts...)
}

// NewPoolWithContext returns a Pool struct
func NewPoolWithContext(ctx context.Context, size int64, opts ...Option) *Pool {
	pool := &Pool{
		countWorkers: int64(size),
		jobChan:      make(chan chan func()),
		system:       make(chan System),
	}
	for _, opt := range opts {
		opt(pool)
	}
	pool.dispatcher(ctx)
	return pool
}

// Do launch async job
func (pool *Pool) Do(job func()) {
	(<-pool.jobChan) <- job
}

// Release shutdown a pool
func (pool *Pool) Release() {
	pool.system <- closingPool
}

// Resize resizes a pool
func (pool *Pool) Resize(size int64) {
	atomic.StoreInt64(&pool.countWorkers, int64(size))
	pool.system <- resize
}

func (w *worker) launch(job func()) {
	defer func() {
		if r := recover(); r != nil {
			w.panicHandler(r)
		}
	}()
	if job != nil {
		job()
	}
}

func (pool *Pool) startWorker() {
	w := &worker{
		jobPoolChan:  pool.jobChan,
		jobChan:      make(chan func()),
		panicHandler: pool.panicHandler,
	}
	go func() {
		defer func() {
			atomic.StoreInt32(&pool.terminateWorker, 0)
			pool.system <- reduce
		}()
	Loop:
		for {
			w.jobPoolChan <- w.jobChan
			data := <-w.jobChan
			if data == nil && atomic.LoadInt32(&pool.terminateWorker) == 1 {
				break Loop
			}
			w.launch(data)
		}
	}()
	pool.system <- enlarge
}

func (pool *Pool) stopWorker() {
	atomic.StoreInt32(&pool.terminateWorker, 1)
	pool.Do(nil)
}

func (pool *Pool) dispatcher(ctx context.Context) {
	go func() {
		tick := time.NewTicker(10 * time.Second)
	Loop:
		for {
			select {
			case code := <-pool.system:
				switch code {
				case resize:
					countWorkers := atomic.LoadInt64(&pool.countWorkers)
					currentCountWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
					if currentCountWorkers < countWorkers {
						go pool.startWorker()
					}
					if currentCountWorkers > countWorkers {
						go pool.stopWorker()
					}
					if currentCountWorkers == 0 && countWorkers == 0 {
						go func() { pool.system <- closedPool }()
					}
				case enlarge:
					atomic.AddInt64(&pool.currentCountWorkers, 1)
					go func() { pool.system <- resize }()
				case reduce:
					atomic.AddInt64(&pool.currentCountWorkers, -1)
					go func() { pool.system <- resize }()
				case closingPool:
					atomic.StoreInt64(&pool.countWorkers, 0)
					go func() { pool.system <- resize }()
				case closedPool:
					// close(pool.jobPoolChan)
					break Loop
				}
			case <-tick.C:
				go func() { pool.system <- resize }()
			case <-ctx.Done():
				atomic.StoreInt64(&pool.countWorkers, 0)
				go func() { pool.system <- resize }()
			}
		}
	}()
	pool.system <- resize
}
