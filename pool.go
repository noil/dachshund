package dachshund

import (
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrReleasePool          = errors.New("can't release pool, try it latter.")
	ErrReloadPoolInProgress = errors.New("can't reload pool, try it latter.")
)

type worker struct {
	job            Pooler
	dispatcherChan chan chan interface{}
	jobQueueChan   chan interface{}
	sync.WaitGroup
	sync.RWMutex
}

type Pool struct {
	job                Pooler
	numOfWorkers       int32
	actualNumOfWorkers int32
	dispatcherChan     chan chan interface{}
	isDisable          int32
	isDisableWorker    int32
	closedChan         chan struct{}
	sync.WaitGroup
	sync.RWMutex
}

func NewPool(number int, job Pooler) *Pool {
	pool := &Pool{
		job:            job,
		numOfWorkers:   int32(number),
		dispatcherChan: make(chan chan interface{}),
		closedChan:     make(chan struct{}),
	}
	pool.dispatcher()

	return pool
}

func (pool *Pool) Do(data interface{}) {
	(<-pool.dispatcherChan) <- data
}

func (pool *Pool) Release() error {
	if 1 == atomic.LoadInt32(&pool.isDisable) {
		return ErrReleasePool
	}
	atomic.AddInt32(&pool.isDisable, 1)
	actualNumOfWorkers := atomic.LoadInt32(&pool.actualNumOfWorkers)
	pool.Add(int(actualNumOfWorkers))
	atomic.SwapInt32(&pool.numOfWorkers, 0)
	pool.Wait()
	pool.Add(1)
	pool.closedChan <- struct{}{}
	pool.Wait()

	return nil
}

func (pool *Pool) Reload(number int, job Pooler) error {
	pool.Lock()
	defer pool.Unlock()
	if 1 == atomic.LoadInt32(&pool.isDisable) {
		return ErrReloadPoolInProgress
	}
	atomic.AddInt32(&pool.isDisable, 1)
	atomic.SwapInt32(&pool.numOfWorkers, int32(number))
	if pool.job != job {
		pool.job = job
	}
	atomic.AddInt32(&pool.isDisable, -1)

	return nil
}

func (pool *Pool) worker() {
	isDisable := atomic.LoadInt32(&pool.isDisable)
	if 1 == isDisable {
		return
	}
	pool.Lock()
	defer pool.Unlock()
	w := &worker{
		job:            pool.job,
		dispatcherChan: pool.dispatcherChan,
		jobQueueChan:   make(chan interface{}),
	}
	atomic.AddInt32(&pool.actualNumOfWorkers, 1)

	go func() {
	Loop:
		for {
			pool.RLock()
			if pool.job != w.job {
				w.Lock()
				w.job = pool.job
				w.Unlock()
			}
			pool.RUnlock()
			w.dispatcherChan <- w.jobQueueChan
			data := <-w.jobQueueChan
			if nil == data && 1 == atomic.LoadInt32(&pool.isDisableWorker) {
				pool.Done()
				break Loop
			}
			w.job.Do(data)
		}
	}()
}

func (pool *Pool) stopWorker() {
	atomic.AddInt32(&pool.isDisableWorker, 1)
	pool.Do(nil)
	atomic.AddInt32(&pool.isDisableWorker, -1)
	atomic.AddInt32(&pool.actualNumOfWorkers, -1)
}

func (pool *Pool) dispatcher() {
	go func() {
	Loop:
		for {
			select {
			case <-pool.closedChan:
				pool.Done()
				break Loop
			default:
				if atomic.LoadInt32(&pool.actualNumOfWorkers) < atomic.LoadInt32(&pool.numOfWorkers) {
					pool.worker()
				} else if atomic.LoadInt32(&pool.actualNumOfWorkers) > atomic.LoadInt32(&pool.numOfWorkers) {
					pool.stopWorker()
				}
			}
		}
	}()
}
