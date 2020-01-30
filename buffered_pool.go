package dachshund

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/pkg/errors"
)

type BufferedPool struct {
	label              string
	task               Task
	numOfWorkers       int32
	actualNumOfWorkers int32
	queueBufferSize    int32
	queueChan          chan interface{}
	closingWorkerChan  chan struct{}
	closedWorkerChan   chan struct{}
	closingChan        chan struct{}
	closedChan         chan struct{}
	resizingChan       chan struct{}
	log                EventReciever
}

// New buffered pool instantiates a BufferedPool
func NewBufferedPool(label string, number, bufferSize int, task Task, log EventReciever) *BufferedPool {
	return NewBufferedPoolWithContext(context.Background(), label, number, bufferSize, task, log)
}

// New buffered pool instantiates a BufferedPool with context
func NewBufferedPoolWithContext(ctx context.Context, label string, number, bufferSize int, task Task, log EventReciever) *BufferedPool {
	if log == nil {
		log = nullReceiver
	}
	pool := &BufferedPool{
		label:             label,
		task:              task,
		numOfWorkers:      int32(number),
		queueBufferSize:   int32(bufferSize),
		queueChan:         make(chan interface{}, bufferSize),
		closingWorkerChan: make(chan struct{}),
		closedWorkerChan:  make(chan struct{}),
		closingChan:       make(chan struct{}),
		closedChan:        make(chan struct{}),
		resizingChan:      make(chan struct{}),
		log:               log,
	}
	pool.dispatcher(ctx)
	return pool
}

// Launch tasks
func (pool *BufferedPool) Do(data interface{}) {
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
			pool.log.EventErrKv(pool.label+".pool.do.task.error", ErrSendOnClosedChannelPanic, kvs)
		}
	}()
	pool.queueChan <- data
}

func (pool *BufferedPool) Release() {
	pool.closingChan <- struct{}{}
	<-pool.closedChan
}

func (pool *BufferedPool) Reload(number int) {
	atomic.SwapInt32(&pool.numOfWorkers, int32(number))
	pool.resizingChan <- struct{}{}
}

func (pool *BufferedPool) startWorker() {
	go func() {
		atomic.AddInt32(&pool.actualNumOfWorkers, 1)
	Loop:
		for {
			select {
			case data, ok := <-pool.queueChan:
				if ok {
					pool.launchTask(data)
				} else {
					//TODO: return event to EventReciecer
				}
			case <-pool.closingWorkerChan:
				atomic.AddInt32(&pool.actualNumOfWorkers, -1)
				pool.closedWorkerChan <- struct{}{}
				break Loop
			}
		}
	}()
}

func (pool *BufferedPool) stopWorker() bool {
	if 0 != atomic.LoadInt32(&pool.actualNumOfWorkers) {
		pool.closingWorkerChan <- struct{}{}
		<-pool.closedWorkerChan

		return true
	}

	return false
}

func (pool *BufferedPool) stopWorkers() {
	atomic.SwapInt32(&pool.numOfWorkers, 0)
	for pool.stopWorker() {
	}
}

func (pool *BufferedPool) launchTask(data interface{}) {
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
			pool.log.EventErrKv(pool.label+".pool.launch.task.error", ErrDoTaskPanic, map[string]string{"problem": message})
		}
	}()
	pool.task(data)
}

func (pool *BufferedPool) dispatcher(ctx context.Context) {
	go func() {
		for {
			if atomic.LoadInt32(&pool.actualNumOfWorkers) < atomic.LoadInt32(&pool.numOfWorkers) {
				pool.startWorker()
			} else if atomic.LoadInt32(&pool.actualNumOfWorkers) > atomic.LoadInt32(&pool.numOfWorkers) {
				pool.stopWorker()
			} else {
				break
			}
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
				for {
					if atomic.LoadInt32(&pool.actualNumOfWorkers) < atomic.LoadInt32(&pool.numOfWorkers) {
						pool.startWorker()
					} else if atomic.LoadInt32(&pool.actualNumOfWorkers) > atomic.LoadInt32(&pool.numOfWorkers) {
						pool.stopWorker()
					} else {
						break
					}
				}
			}
		}
	}()
}
