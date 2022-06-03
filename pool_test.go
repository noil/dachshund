package dachshund

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRelease(t *testing.T) {
	size := int64(10)
	pool := NewPool(size)
	countWorkers := atomic.LoadInt64(&pool.countWorkers)
	assert.Equal(t, size, countWorkers)

	pool.Release()
	time.Sleep(1 * time.Second)
	countWorkers = atomic.LoadInt64(&pool.countWorkers)
	assert.Equal(t, int64(0), countWorkers)
}

func TestResize(t *testing.T) {
	size := int64(10)
	pool := NewPool(size)
	countWorkers := atomic.LoadInt64(&pool.countWorkers)
	assert.Equal(t, size, countWorkers)

	size = 12
	pool.Resize(size)
	time.Sleep(1 * time.Second)
	countWorkers = atomic.LoadInt64(&pool.countWorkers)
	assert.Equal(t, size, countWorkers)

	size = 8
	pool.Resize(size)
	time.Sleep(1 * time.Second)
	countWorkers = atomic.LoadInt64(&pool.countWorkers)
	assert.Equal(t, size, countWorkers)
}

func TestDoWithPanicHandler(t *testing.T) {
	size := int64(10)
	var message string
	pool := NewPool(size, WithPanicHandler(func(r any) {
		message = fmt.Sprintf(`%v`, r)
	}))

	pool.Do(func() {
		panic("foo")
	})

	time.Sleep(1 * time.Second)
	assert.Equal(t, "foo", message)
}

func TestDo(t *testing.T) {
	sum := make(chan int)

	size := int64(10)
	pool := NewPool(size)
	go func() {
		defer close(sum)
		for i := 0; i < 100000; i++ {
			j := i
			pool.Do(func() {
				sum <- j
			})
		}
	}()
	result := 0
	for i := range sum {
		result += i
	}
	assert.Equal(t, 4999950000, result)
}

func TestStartWorker(t *testing.T) {
	size := int64(10)
	pool := NewPool(size)
	countWorkers := atomic.LoadInt64(&pool.countWorkers)
	assert.Equal(t, size, countWorkers)
	pool.startWorker()
	time.Sleep(1 * time.Second)
	countWorkers = atomic.LoadInt64(&pool.countWorkers)
	assert.Equal(t, size, countWorkers)
}

func TestStopWorker(t *testing.T) {
	size := int64(10)
	pool := NewPool(size)
	countWorkers := atomic.LoadInt64(&pool.countWorkers)
	assert.Equal(t, size, countWorkers)
	pool.stopWorker()
	time.Sleep(1 * time.Second)
	countWorkers = atomic.LoadInt64(&pool.countWorkers)
	assert.Equal(t, size, countWorkers)
}
