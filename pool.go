package dachshund

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
)

type worker struct {
	label          string
	task           Task
	dispatcherChan chan chan interface{}
	taskQueueChan  chan interface{}
	log            EventReciever
	closingChan    chan struct{}
	closedChan     chan struct{}
}

type Pool struct {
	label              string
	task               Task
	numOfWorkers       int32
	actualNumOfWorkers int32
	dispatcherChan     chan chan interface{}
	resizingChan       chan struct{}
	closingChan        chan struct{}
	closedChan         chan struct{}
	workerClosingChan  chan struct{}
	workerClosedChan   chan struct{}
	log                EventReciever
	mu                 sync.RWMutex
}

func NewPool(label string, number int, task Task, log EventReciever) *Pool {
	return NewPoolWithContext(context.Background(), label, number, task, log)
}

func NewPoolWithContext(ctx context.Context, label string, number int, task Task, log EventReciever) *Pool {
	if log == nil {
		log = nullReceiver
	}
	pool := &Pool{
		label:             label,
		task:              task,
		numOfWorkers:      int32(number),
		dispatcherChan:    make(chan chan interface{}),
		resizingChan:      make(chan struct{}),
		closingChan:       make(chan struct{}),
		closedChan:        make(chan struct{}),
		workerClosingChan: make(chan struct{}),
		workerClosedChan:  make(chan struct{}),
		log:               log,
	}
	pool.dispatcher(ctx)
	return pool
}

func (pool *Pool) Do(data interface{}) {
	defer func() {
		if r := recover(); r != nil {
			var message string
			switch x := r.(type) {
			case string:
				message = x
			case error:
				message = fmt.Sprintf("%+v", errors.WithStack(x))
			default:
				message = fmt.Sprintf("%+v", r)
			}
			kvs := make(map[string]string)
			kvs["problem"] = message
			pool.log.EventErrKv(pool.label+"pool.do.task.error", ErrSendOnClosedChannelPanic, kvs)
		}
	}()
	(<-pool.dispatcherChan) <- data
}

func (pool *Pool) Release() {
	pool.closingChan <- struct{}{}
	<-pool.closedChan
}

func (pool *Pool) Reload(number int) {
	pool.mu.Lock()
	pool.numOfWorkers = int32(number)
	pool.mu.Unlock()
	// atomic.SwapInt32(&pool.numOfWorkers, int32(number))
	pool.resizingChan <- struct{}{}
}

func (pool *Pool) startWorker() {
	w := &worker{
		label:          pool.label,
		task:           pool.task,
		dispatcherChan: pool.dispatcherChan,
		taskQueueChan:  make(chan interface{}),
		log:            pool.log,
		closingChan:    pool.workerClosingChan,
		closedChan:     pool.workerClosedChan,
	}
	pool.mu.Lock()
	pool.actualNumOfWorkers += 1
	pool.mu.Unlock()
	// atomic.AddInt32(&pool.actualNumOfWorkers, 1)

	go func() {
	Loop:
		for {
			w.dispatcherChan <- w.taskQueueChan
			select {
			case data := <-w.taskQueueChan:
				w.launchTask(data)
			case <-w.closingChan:
				w.closedChan <- struct{}{}
				break Loop
			}
		}
		// atomic.AddInt32(&pool.actualNumOfWorkers, -1)
	}()
}

func (w *worker) launchTask(data interface{}) {
	defer func() {
		if r := recover(); r != nil {
			var message string
			switch x := r.(type) {
			case string:
				message = x
			case error:
				message = fmt.Sprintf("%+v", errors.WithStack(x))
			default:
				message = fmt.Sprintf("%+v", r)
			}
			w.log.EventErrKv(w.label+".pool.launch.task.error", ErrDoTaskPanic, map[string]string{"problem": message})
		}
	}()
	w.task(data)
}

func (pool *Pool) stopWorker() bool {
	var result bool
	pool.mu.Lock()
	// if 0 != atomic.LoadInt32(&pool.actualNumOfWorkers) {
	if 0 != pool.actualNumOfWorkers {
		<-pool.dispatcherChan
		pool.workerClosingChan <- struct{}{}
		<-pool.workerClosedChan
		pool.actualNumOfWorkers -= 1
		result = true
	}
	pool.mu.Unlock()
	return result
}

func (pool *Pool) stopWorkers() {
	pool.mu.Lock()
	pool.numOfWorkers = 0
	pool.mu.Unlock()
	// atomic.SwapInt32(&pool.numOfWorkers, 0)
	for pool.stopWorker() {
	}
}

func (pool *Pool) dispatcher(ctx context.Context) {
	go func() {
		pool.mu.Lock()
		// actualNumOfWorkers := atomic.LoadInt32(&pool.actualNumOfWorkers)
		// numOfWorkers := atomic.LoadInt32(&pool.numOfWorkers)
		actualNumOfWorkers := pool.actualNumOfWorkers
		numOfWorkers := pool.numOfWorkers
		pool.mu.Unlock()
		for actualNumOfWorkers < numOfWorkers {
			actualNumOfWorkers++
			pool.startWorker()
		}
	Loop:
		for {
			select {
			case <-pool.closingChan:
				pool.stopWorkers()
				pool.closedChan <- struct{}{}
				break Loop
			case <-ctx.Done():
				pool.stopWorkers()
				break Loop
			case <-pool.resizingChan:
				pool.mu.Lock()
				actualNumOfWorkers := pool.actualNumOfWorkers
				numOfWorkers := pool.numOfWorkers
				pool.mu.Unlock()
				for {
					if actualNumOfWorkers < numOfWorkers {
						actualNumOfWorkers++
						pool.startWorker()
					} else if actualNumOfWorkers > numOfWorkers {
						actualNumOfWorkers--
						pool.stopWorker()
					} else {
						break
					}
				}
				// for {
				// 	if atomic.LoadInt32(&pool.actualNumOfWorkers) < atomic.LoadInt32(&pool.numOfWorkers) {
				// 		pool.startWorker()
				// 	} else if atomic.LoadInt32(&pool.actualNumOfWorkers) > atomic.LoadInt32(&pool.numOfWorkers) {
				// 		pool.stopWorker()
				// 	} else {
				// 		break
				// 	}
				// }
			}
		}
	}()
}
