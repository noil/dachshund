package dachshund

import (
	"context"
	"fmt"
	"sync"

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
	mu                 sync.RWMutex
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
	pool.mu.Lock()
	pool.numOfWorkers = int32(number)
	pool.mu.Unlock()
	pool.resizingChan <- struct{}{}
}

func (pool *BufferedPool) startWorker() {
	go func() {
		pool.mu.Lock()
		pool.actualNumOfWorkers += 1
		pool.mu.Unlock()
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
				pool.closedWorkerChan <- struct{}{}
				break Loop
			}
		}
	}()
}

func (pool *BufferedPool) stopWorker() bool {
	var result bool
	pool.mu.Lock()
	if 0 != pool.actualNumOfWorkers {
		pool.closingWorkerChan <- struct{}{}
		<-pool.closedWorkerChan
		pool.actualNumOfWorkers -= 1
		result = true
	}
	pool.mu.Unlock()
	return result
}

func (pool *BufferedPool) stopWorkers() {
	pool.mu.Lock()
	pool.numOfWorkers = 0
	pool.mu.Unlock()
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
		pool.mu.Lock()
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
			}
		}
	}()
}
