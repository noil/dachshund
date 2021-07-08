package dachshund

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
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
	callChan     chan func()
	callPoolChan chan chan func()
	log          EventReciever
}

type Pool struct {
	callChan            chan chan func()
	countWorkers        int64
	currentCountWorkers int64
	system              chan System
	log                 EventReciever
	terminateWorker     int32
}

func NewPool(number int, log EventReciever) *Pool {
	return NewPoolWithContext(context.Background(), number, log)
}

func NewPoolWithContext(ctx context.Context, number int, log EventReciever) *Pool {
	if log == nil {
		log = nullReceiver
	}

	pool := &Pool{
		countWorkers: int64(number),
		callChan:     make(chan chan func()),
		system:       make(chan System),
		log:          log,
	}
	pool.dispatcher(ctx)
	return pool
}

func (pool *Pool) Do(call func()) {
	(<-pool.callChan) <- call
}

func (pool *Pool) Release() {
	pool.system <- closingPool
}

func (pool *Pool) Resize(number int) {
	atomic.StoreInt64(&pool.countWorkers, int64(number))
	pool.system <- resize
}

func (w *worker) launch(call func()) {
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
			err := w.log.EventErrKv("dachshund: panic", ErrDoPanic, map[string]string{"error": message})
			if err != nil {
				panic(err)
			}
		}
	}()
	if call != nil {
		call()
	}
}

func (pool *Pool) startWorker() {
	w := &worker{
		callPoolChan: pool.callChan,
		callChan:     make(chan func()),
		log:          pool.log,
	}
	go func() {
		defer func() {
			atomic.StoreInt32(&pool.terminateWorker, 0)
			pool.system <- reduce
		}()
	Loop:
		for {
			w.callPoolChan <- w.callChan
			data := <-w.callChan
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
					// close(pool.callPoolChan)
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
