package dachshund

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const (
	resize = iota + 1
	enlarge
	reduce
	closeWorker
	closingPool
	closedPool
)

type System int
type worker struct {
	jobChan       chan func()
	jobPoolChan   chan chan func()
	terminateChan chan struct{}
	panicHandler  func(any)
}

type Pool struct {
	jobChan             chan chan func()
	countWorkers        int64
	currentCountWorkers int64
	system              chan System
	terminateWorkerChan chan struct{}
	closeChan           chan struct{}
	panicHandler        func(any)
	mu                  sync.RWMutex
	closed              bool
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
		countWorkers:        int64(size),
		jobChan:             make(chan chan func()),
		system:              make(chan System, 1000),
		terminateWorkerChan: make(chan struct{}),
		closeChan:           make(chan struct{}),
	}
	for _, opt := range opts {
		opt(pool)
	}
	pool.dispatcher(ctx)
	return pool
}

// Do launch async job
func (pool *Pool) Do(job func()) {
	defer func() {
		recover()
	}()
	select {
	case <-pool.closeChan:
		return
	case workerChan := <-pool.jobChan:
		workerChan <- job
	}
}

// Release shutdown a pool
func (pool *Pool) Release() {
	pool.sendSystemSignal(closingPool)
}

// Resize resizes a pool
func (pool *Pool) Resize(size int64) {
	atomic.StoreInt64(&pool.countWorkers, int64(size))
	pool.sendSystemSignal(resize)
}

func (pool *Pool) sendSystemSignal(sig System) {
	pool.mu.RLock()
	if pool.closed {
		pool.mu.RUnlock()
		return
	}
	defer func() {
		pool.mu.RUnlock()
		recover()
	}()
	pool.system <- sig
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
		jobPoolChan:   pool.jobChan,
		terminateChan: pool.terminateWorkerChan,
		jobChan:       make(chan func()),
		panicHandler:  pool.panicHandler,
	}
	go func() {
		defer func() {
			recover()
			pool.sendSystemSignal(reduce)
		}()
	Loop:
		for {
			select {
			case <-pool.closeChan:
				break Loop
			case <-w.terminateChan:
				break Loop
			case w.jobPoolChan <- w.jobChan:
				data := <-w.jobChan
				w.launch(data)
			}
		}
	}()
	pool.sendSystemSignal(enlarge)
}

func (pool *Pool) stopWorker() {
	pool.terminateWorkerChan <- struct{}{}
}

func (pool *Pool) dispatcher(ctx context.Context) {
	go func() {
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
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
						go pool.sendSystemSignal(closedPool)
					}
				case enlarge:
					atomic.AddInt64(&pool.currentCountWorkers, 1)
					go pool.sendSystemSignal(resize)
				case reduce:
					atomic.AddInt64(&pool.currentCountWorkers, -1)
					go pool.sendSystemSignal(resize)
				case closingPool:
					atomic.StoreInt64(&pool.countWorkers, 0)
					go pool.sendSystemSignal(resize)
				case closedPool:
					pool.mu.Lock()
					pool.closed = true
					close(pool.closeChan)
					close(pool.system)
					pool.mu.Unlock()
					return
				}
			case <-tick.C:
				pool.sendSystemSignal(resize)
			case <-ctx.Done():
				atomic.StoreInt64(&pool.countWorkers, 0)
				pool.sendSystemSignal(resize)
			}
		}
	}()
	pool.sendSystemSignal(resize)
}
