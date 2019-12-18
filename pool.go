package dachshund

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/pkg/errors"
)

type worker struct {
	label          string
	task           Task
	dispatcherChan chan chan interface{}
	taskQueueChan  chan interface{}
	log            EventReciever
}

type Pool struct {
	label              string
	task               Task
	numOfWorkers       int32
	actualNumOfWorkers int32
	dispatcherChan     chan chan interface{}
	isDisableWorker    int32
	closingChan        chan struct{}
	closedChan         chan struct{}
	log                EventReciever
}

func NewPool(label string, number int, task Task, log EventReciever) *Pool {
	return NewPoolWithContext(context.Background(), label, number, task, log)
}

func NewPoolWithContext(ctx context.Context, label string, number int, task Task, log EventReciever) *Pool {
	pool := &Pool{
		label:          label,
		task:           task,
		numOfWorkers:   int32(number),
		dispatcherChan: make(chan chan interface{}),
		closingChan:    make(chan struct{}),
		closedChan:     make(chan struct{}),
		log:            log,
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
	atomic.SwapInt32(&pool.numOfWorkers, int32(number))
}

func (pool *Pool) startWorker() {
	w := &worker{
		label:          pool.label,
		task:           pool.task,
		dispatcherChan: pool.dispatcherChan,
		taskQueueChan:  make(chan interface{}),
		log:            pool.log,
	}
	atomic.AddInt32(&pool.actualNumOfWorkers, 1)

	go func() {
		defer func() {
			atomic.AddInt32(&pool.actualNumOfWorkers, -1)
		}()
		for {
			w.dispatcherChan <- w.taskQueueChan
			data := <-w.taskQueueChan
			// close worker
			if nil == data && 1 == atomic.LoadInt32(&pool.isDisableWorker) {
				break
			} else {
				w.launchTask(data)
			}
		}
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
			w.log.EventErrKv(w.label+"pool.launch.task.error", ErrDoTaskPanic, map[string]string{"problem": message})
		}
	}()
	w.task(data)
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
			case <-pool.closingChan:
				atomic.SwapInt32(&pool.numOfWorkers, 0)
				for {
					if 0 != atomic.LoadInt32(&pool.actualNumOfWorkers) {
						pool.stopWorker()
					} else {
						break
					}
				}
				pool.closedChan <- struct{}{}
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
					pool.startWorker()
				} else if atomic.LoadInt32(&pool.actualNumOfWorkers) > atomic.LoadInt32(&pool.numOfWorkers) {
					pool.stopWorker()
				}
			}
		}
	}()
}
