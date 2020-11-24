package dachshund

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
)

type FuncPool struct {
	label              string
	numOfWorkers       int32
	actualNumOfWorkers int32
	fChan              chan chan func()
	resizingChan       chan struct{}
	closingChan        chan struct{}
	closedChan         chan struct{}
	workerClosingChan  chan struct{}
	workerClosedChan   chan struct{}
	log                EventReciever
	mu                 sync.RWMutex
}

func NewFuncPool(label string, number int, log EventReciever) *FuncPool {
	return NewFuncPoolWithContext(context.Background(), label, number, log)
}

func NewFuncPoolWithContext(ctx context.Context, label string, number int, log EventReciever) *FuncPool {
	if log == nil {
		log = nullReceiver
	}
	pool := &FuncPool{
		label:             label,
		numOfWorkers:      int32(number),
		fChan:             make(chan chan func()),
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

func (pool *FuncPool) Do(f func()) {
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
			pool.log.EventErrKv(pool.label+"func.pool.do.task.error", ErrSendOnClosedChannelPanic, kvs)
		}
	}()
	(<-pool.fChan) <- f
}

func (pool *FuncPool) Release() {
	pool.closingChan <- struct{}{}
	<-pool.closedChan
}

func (pool *FuncPool) Reload(number int) {
	pool.mu.Lock()
	pool.numOfWorkers = int32(number)
	pool.mu.Unlock()
	pool.resizingChan <- struct{}{}
}

func (pool *FuncPool) startWorker() {
	w := &worker{
		label:       pool.label,
		fChan:       pool.fChan,
		fQueueChan:  make(chan func()),
		log:         pool.log,
		closingChan: pool.workerClosingChan,
		closedChan:  pool.workerClosedChan,
	}
	pool.mu.Lock()
	pool.actualNumOfWorkers++
	pool.mu.Unlock()

	go func() {
	Loop:
		for {
			w.fChan <- w.fQueueChan
			select {
			case f := <-w.fQueueChan:
				w.launchFunc(f)
			case <-w.closingChan:
				w.closedChan <- struct{}{}
				break Loop
			}
		}
	}()
}

func (w *worker) launchFunc(f func()) {
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
	f()
}

func (pool *FuncPool) stopWorker() bool {
	var result bool
	pool.mu.Lock()
	if 0 != pool.actualNumOfWorkers {
		<-pool.fChan
		pool.workerClosingChan <- struct{}{}
		<-pool.workerClosedChan
		pool.actualNumOfWorkers--
		result = true
	}
	pool.mu.Unlock()
	return result
}

func (pool *FuncPool) stopWorkers() {
	pool.mu.Lock()
	pool.numOfWorkers = 0
	pool.mu.Unlock()
	for pool.stopWorker() {
	}
}

func (pool *FuncPool) dispatcher(ctx context.Context) {
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
