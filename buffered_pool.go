package dachshund

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrReleaseBufferedPool          = errors.New("can't release pool, try it latter.")
	ErrReloadBufferedPoolInProgress = errors.New("can't reload pool, try it latter.")
)

type BufferedPool struct {
	job                Pooler
	numOfWorkers       int32
	actualNumOfWorkers int32
	jobQueueBufferSize int32
	isDisable          int32
	isEmptyChan        int32
	jobQueueChan       chan interface{}
	closedChan         chan struct{}
	sync.WaitGroup
	sync.RWMutex
}

func NewBufferedPool(number, jobBufferSize int, job Pooler) *BufferedPool {
	pool := &BufferedPool{
		job:                job,
		numOfWorkers:       int32(number),
		jobQueueBufferSize: int32(jobBufferSize),
		jobQueueChan:       make(chan interface{}, jobBufferSize),
		closedChan:         make(chan struct{}),
	}
	pool.dispatcher()

	return pool
}

func (pool *BufferedPool) Do(data interface{}) {
	if data != nil {
		for 0 < atomic.LoadInt32(&pool.isDisable) {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if 0 == atomic.LoadInt32(&pool.isEmptyChan) {
		pool.jobQueueChan <- data
	}
}

func (pool *BufferedPool) Release() error {
	isDisable := atomic.LoadInt32(&pool.isDisable)
	if 1 == isDisable {
		return ErrReleaseBufferedPool
	}
	atomic.AddInt32(&pool.isDisable, 1)
	numOfWorkers := atomic.LoadInt32(&pool.actualNumOfWorkers)
	atomic.SwapInt32(&pool.numOfWorkers, 0)
	pool.Add(int(numOfWorkers))
	for i := int32(0); i < numOfWorkers; i++ {
		pool.Do(nil)
	}
	pool.Wait()
	atomic.AddInt32(&pool.isEmptyChan, 1)
	close(pool.jobQueueChan)
	pool.Add(1)
	pool.closedChan <- struct{}{}
	pool.Wait()

	return nil
}

func (pool *BufferedPool) Reload(number, jobBufferSize int, job Pooler) error {
	pool.Lock()
	isDisable := atomic.LoadInt32(&pool.isDisable)
	if 1 == isDisable {
		return ErrReloadBufferedPoolInProgress
	}
	defer func() {
		pool.Unlock()
	}()
	atomic.AddInt32(&pool.isDisable, 1)
	pool.Add(1)
	pool.closedChan <- struct{}{}
	pool.Wait()
	actualNumOfWorkers := atomic.LoadInt32(&pool.actualNumOfWorkers)
	atomic.SwapInt32(&pool.numOfWorkers, 0)
	pool.Add(int(actualNumOfWorkers))
	for i := int32(0); i < actualNumOfWorkers; i++ {
		pool.Do(nil)
	}
	pool.Wait()
	atomic.AddInt32(&pool.isEmptyChan, 1)
	atomic.SwapInt32(&pool.numOfWorkers, int32(number))
	if pool.job != job {
		pool.job = job
	}
	close(pool.jobQueueChan)
	pool.jobQueueChan = make(chan interface{}, jobBufferSize)
	atomic.AddInt32(&pool.isEmptyChan, -1)
	atomic.AddInt32(&pool.isDisable, -1)
	pool.dispatcher()

	return nil
}

func (pool *BufferedPool) worker() {
	defer func() {
		atomic.AddInt32(&pool.actualNumOfWorkers, -1)
	}()
	for data := range pool.jobQueueChan {
		/**
		* close worker
		 */
		if nil == data && 1 == atomic.LoadInt32(&pool.isDisable) {
			pool.Done()
			return
		}
		pool.doTask(data)
	}
}

func (pool *BufferedPool) doTask(data interface{}) {
	defer func() {
		_ = recover()
		// if r := recover(); r != nil {
		//   fmt.Printf("Task has been failed. Reciving data - %v. Error - %s\n", data, r)
		// }
	}()
	pool.job.Do(data)
}

func (pool *BufferedPool) dispatcher() {
	go func() {
	Loop:
		for {
			select {
			case <-pool.closedChan:
				pool.Done()
				break Loop
			default:
				if atomic.LoadInt32(&pool.actualNumOfWorkers) < atomic.LoadInt32(&pool.numOfWorkers) {
					go pool.worker()
					atomic.AddInt32(&pool.actualNumOfWorkers, 1)
				}
			}
		}
	}()
}
