package dachshund

import (
	"context"
	"sync/atomic"
)

type worker struct {
	job            Pooler
	dispatcherChan chan chan interface{}
	jobQueueChan   chan interface{}
}

type Pool struct {
	job                 Pooler
	numOfWorkers        int32
	actualNumOfWorkers  int32
	dispatcherChan      chan chan interface{}
	isDisableWorker     int32
	determinatePoolChan chan struct{}
	closedPoolChan      chan struct{}
}

func NewPool(number int, job Pooler) *Pool {
	return NewPoolWithContext(context.Background(), number, job)
}

func NewPoolWithContext(ctx context.Context, number int, job Pooler) *Pool {
	pool := &Pool{
		job:                 job,
		numOfWorkers:        int32(number),
		dispatcherChan:      make(chan chan interface{}),
		determinatePoolChan: make(chan struct{}),
		closedPoolChan:      make(chan struct{}),
	}
	pool.dispatcher(ctx)

	return pool
}

func (pool *Pool) Do(data interface{}) {
	(<-pool.dispatcherChan) <- data
}

func (pool *Pool) Release() {
	pool.determinatePoolChan <- struct{}{}
	<-pool.closedPoolChan
}

func (pool *Pool) Reload(number int) {
	atomic.SwapInt32(&pool.numOfWorkers, int32(number))
}

func (pool *Pool) startWorker(ctx context.Context) {
	w := &worker{
		job:            pool.job,
		dispatcherChan: pool.dispatcherChan,
		jobQueueChan:   make(chan interface{}),
	}
	atomic.AddInt32(&pool.actualNumOfWorkers, 1)

	go func() {
	Loop:
		for {
			w.dispatcherChan <- w.jobQueueChan
			data := <-w.jobQueueChan
			if nil == data && 1 == atomic.LoadInt32(&pool.isDisableWorker) {
				atomic.AddInt32(&pool.actualNumOfWorkers, -1)
				break Loop
			} else {
				w.launchTask(data)
			}
		}
	}()
}

func (w *worker) launchTask(data interface{}) {
	defer func() {
		_ = recover()
	}()
	w.job.Do(data)
}

func (pool *Pool) stopWorker() {
	atomic.SwapInt32(&pool.isDisableWorker, 1)
	defer atomic.SwapInt32(&pool.isDisableWorker, 0)
	pool.Do(nil)
}

func (pool *Pool) dispatcher(ctx context.Context) {
	go func() {
	Loop:
		for {
			select {
			case <-pool.determinatePoolChan:
				atomic.SwapInt32(&pool.numOfWorkers, 0)
				for {
					if 0 != atomic.LoadInt32(&pool.actualNumOfWorkers) {
						pool.stopWorker()
					} else {
						break
					}
				}
				pool.closedPoolChan <- struct{}{}
				break Loop
			case <-ctx.Done():
				atomic.SwapInt32(&pool.numOfWorkers, 0)
				for {
					if 0 != atomic.LoadInt32(&pool.actualNumOfWorkers) {
						pool.stopWorker()
					} else {
						break
					}
				}
				break Loop
			default:
				if atomic.LoadInt32(&pool.actualNumOfWorkers) < atomic.LoadInt32(&pool.numOfWorkers) {
					pool.startWorker(ctx)
				} else if atomic.LoadInt32(&pool.actualNumOfWorkers) > atomic.LoadInt32(&pool.numOfWorkers) {
					pool.stopWorker()
				}
			}
		}
	}()
}
