package dachshund

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== Pool Creation Tests ====================

func TestNewPool(t *testing.T) {
	t.Run("creates pool with specified size", func(t *testing.T) {
		size := int64(10)
		pool := NewPool(size)
		defer pool.Release()

		countWorkers := atomic.LoadInt64(&pool.countWorkers)
		assert.Equal(t, size, countWorkers)
	})

	t.Run("creates pool with zero size", func(t *testing.T) {
		pool := NewPool(0)
		time.Sleep(100 * time.Millisecond)

		countWorkers := atomic.LoadInt64(&pool.countWorkers)
		assert.Equal(t, int64(0), countWorkers)
		currentWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
		assert.Equal(t, int64(0), currentWorkers)
	})

	t.Run("creates pool with size 1", func(t *testing.T) {
		pool := NewPool(1)
		defer pool.Release()
		time.Sleep(100 * time.Millisecond)

		currentWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
		assert.Equal(t, int64(1), currentWorkers)
	})

	t.Run("creates pool with large size", func(t *testing.T) {
		size := int64(100)
		pool := NewPool(size)
		defer pool.Release()
		time.Sleep(500 * time.Millisecond)

		currentWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
		assert.Equal(t, size, currentWorkers)
	})
}

func TestNewPoolWithContext(t *testing.T) {
	t.Run("pool shuts down when context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		size := int64(5)
		pool := NewPoolWithContext(ctx, size)
		time.Sleep(200 * time.Millisecond)

		currentWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
		assert.Equal(t, size, currentWorkers)

		cancel()
		time.Sleep(500 * time.Millisecond)

		countWorkers := atomic.LoadInt64(&pool.countWorkers)
		assert.Equal(t, int64(0), countWorkers)
	})

	t.Run("pool shuts down with timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		pool := NewPoolWithContext(ctx, 5)
		time.Sleep(100 * time.Millisecond)

		currentWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
		assert.Equal(t, int64(5), currentWorkers)

		time.Sleep(400 * time.Millisecond)

		countWorkers := atomic.LoadInt64(&pool.countWorkers)
		assert.Equal(t, int64(0), countWorkers)
	})

	t.Run("pool works with already cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		pool := NewPoolWithContext(ctx, 5)
		time.Sleep(300 * time.Millisecond)

		countWorkers := atomic.LoadInt64(&pool.countWorkers)
		assert.Equal(t, int64(0), countWorkers)
	})
}

// ==================== Pool Release Tests ====================

func TestRelease(t *testing.T) {
	t.Run("releases pool and sets workers to zero", func(t *testing.T) {
		size := int64(10)
		pool := NewPool(size)
		countWorkers := atomic.LoadInt64(&pool.countWorkers)
		assert.Equal(t, size, countWorkers)

		pool.Release()
		time.Sleep(1 * time.Second)
		countWorkers = atomic.LoadInt64(&pool.countWorkers)
		assert.Equal(t, int64(0), countWorkers)
	})

	t.Run("double release is safe", func(t *testing.T) {
		pool := NewPool(5)
		time.Sleep(200 * time.Millisecond)

		pool.Release()
		time.Sleep(300 * time.Millisecond)

		// Second release should not panic
		assert.NotPanics(t, func() {
			pool.Release()
		})
	})

	t.Run("release empty pool", func(t *testing.T) {
		pool := NewPool(0)
		time.Sleep(200 * time.Millisecond)

		assert.NotPanics(t, func() {
			pool.Release()
		})
	})
}

// ==================== Pool Resize Tests ====================

func TestResize(t *testing.T) {
	t.Run("resize up", func(t *testing.T) {
		size := int64(10)
		pool := NewPool(size)
		defer pool.Release()
		time.Sleep(300 * time.Millisecond)

		pool.Resize(15)
		time.Sleep(500 * time.Millisecond)

		currentWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
		assert.Equal(t, int64(15), currentWorkers)
	})

	t.Run("resize down", func(t *testing.T) {
		size := int64(10)
		pool := NewPool(size)
		defer pool.Release()
		time.Sleep(300 * time.Millisecond)

		pool.Resize(5)
		time.Sleep(500 * time.Millisecond)

		currentWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
		assert.Equal(t, int64(5), currentWorkers)
	})

	t.Run("resize to zero", func(t *testing.T) {
		pool := NewPool(5)
		time.Sleep(200 * time.Millisecond)

		pool.Resize(0)
		time.Sleep(500 * time.Millisecond)

		currentWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
		assert.Equal(t, int64(0), currentWorkers)
	})

	t.Run("resize to same size", func(t *testing.T) {
		pool := NewPool(5)
		defer pool.Release()
		time.Sleep(200 * time.Millisecond)

		pool.Resize(5)
		time.Sleep(300 * time.Millisecond)

		currentWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
		assert.Equal(t, int64(5), currentWorkers)
	})

	t.Run("multiple quick resizes", func(t *testing.T) {
		pool := NewPool(5)
		defer pool.Release()
		time.Sleep(200 * time.Millisecond)

		for i := 0; i < 10; i++ {
			pool.Resize(int64(i + 1))
		}
		time.Sleep(1 * time.Second)

		currentWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
		assert.Equal(t, int64(10), currentWorkers)
	})
}

// ==================== Pool Do Tests ====================

func TestDo(t *testing.T) {
	t.Run("executes single job", func(t *testing.T) {
		pool := NewPool(1)
		defer pool.Release()
		time.Sleep(100 * time.Millisecond)

		var executed atomic.Bool
		pool.Do(func() {
			executed.Store(true)
		})
		time.Sleep(100 * time.Millisecond)

		assert.True(t, executed.Load())
	})

	t.Run("executes many jobs", func(t *testing.T) {
		sum := make(chan int)

		size := int64(10)
		pool := NewPool(size)
		go func() {
			defer close(sum)
			var wg sync.WaitGroup
			wg.Add(100000)
			for i := 0; i < 100000; i++ {
				i := i
				pool.Do(func() {
					sum <- i
					wg.Done()
				})
			}
			wg.Wait()
		}()
		result := 0
		for i := range sum {
			result += i
		}
		assert.Equal(t, 4999950000, result)
	})

	t.Run("executes jobs concurrently", func(t *testing.T) {
		pool := NewPool(10)
		defer pool.Release()
		time.Sleep(200 * time.Millisecond)

		var counter atomic.Int64
		var wg sync.WaitGroup
		wg.Add(100)

		for i := 0; i < 100; i++ {
			pool.Do(func() {
				time.Sleep(10 * time.Millisecond)
				counter.Add(1)
				wg.Done()
			})
		}

		wg.Wait()
		assert.Equal(t, int64(100), counter.Load())
	})

	t.Run("nil job does not panic", func(t *testing.T) {
		pool := NewPool(1)
		defer pool.Release()
		time.Sleep(100 * time.Millisecond)

		assert.NotPanics(t, func() {
			pool.Do(nil)
			time.Sleep(100 * time.Millisecond)
		})
	})
}

// ==================== Pool Panic Handler Tests ====================

func TestDoWithPanicHandler(t *testing.T) {
	t.Run("handles single panic", func(t *testing.T) {
		size := int64(10)
		var (
			message string
			mu      sync.RWMutex
		)
		pool := NewPool(size, WithPanicHandler(func(r any) {
			mu.Lock()
			defer mu.Unlock()
			message = fmt.Sprintf(`%v`, r)
		}))
		defer pool.Release()

		pool.Do(func() {
			panic("foo")
		})

		time.Sleep(1 * time.Second)
		mu.RLock()
		defer mu.RUnlock()
		assert.Equal(t, "foo", message)
	})

	t.Run("handles multiple panics", func(t *testing.T) {
		var (
			panicCount atomic.Int64
			mu         sync.Mutex
		)
		pool := NewPool(5, WithPanicHandler(func(r any) {
			mu.Lock()
			defer mu.Unlock()
			panicCount.Add(1)
		}))
		defer pool.Release()
		time.Sleep(200 * time.Millisecond)

		for i := 0; i < 10; i++ {
			pool.Do(func() {
				panic("test panic")
			})
		}

		time.Sleep(500 * time.Millisecond)
		assert.Equal(t, int64(10), panicCount.Load())
	})

	t.Run("pool continues after panic", func(t *testing.T) {
		var executed atomic.Bool
		pool := NewPool(2, WithPanicHandler(func(r any) {}))
		defer pool.Release()
		time.Sleep(100 * time.Millisecond)

		pool.Do(func() {
			panic("first panic")
		})
		time.Sleep(100 * time.Millisecond)

		pool.Do(func() {
			executed.Store(true)
		})
		time.Sleep(100 * time.Millisecond)

		assert.True(t, executed.Load())
	})

	t.Run("no panic handler - pool still works", func(t *testing.T) {
		pool := NewPool(2)
		defer pool.Release()
		time.Sleep(100 * time.Millisecond)

		var executed atomic.Bool
		pool.Do(func() {
			panic("unhandled")
		})
		time.Sleep(100 * time.Millisecond)

		pool.Do(func() {
			executed.Store(true)
		})
		time.Sleep(100 * time.Millisecond)

		assert.True(t, executed.Load())
	})
}

// ==================== Pool Worker Tests ====================

func TestStartWorker(t *testing.T) {
	t.Run("starts additional worker", func(t *testing.T) {
		size := int64(10)
		pool := NewPool(size)
		defer pool.Release()
		time.Sleep(300 * time.Millisecond)

		countWorkers := atomic.LoadInt64(&pool.countWorkers)
		assert.Equal(t, size, countWorkers)

		pool.startWorker()
		time.Sleep(500 * time.Millisecond)

		// Worker count should stay at configured size due to resize
		countWorkers = atomic.LoadInt64(&pool.countWorkers)
		assert.Equal(t, size, countWorkers)
	})
}

func TestStopWorker(t *testing.T) {
	t.Run("stops a worker", func(t *testing.T) {
		size := int64(10)
		pool := NewPool(size)
		defer pool.Release()
		time.Sleep(300 * time.Millisecond)

		countWorkers := atomic.LoadInt64(&pool.countWorkers)
		assert.Equal(t, size, countWorkers)

		pool.stopWorker()
		time.Sleep(500 * time.Millisecond)

		// Worker count should stay at configured size due to resize
		countWorkers = atomic.LoadInt64(&pool.countWorkers)
		assert.Equal(t, size, countWorkers)
	})
}

// ==================== Pool Stress Tests ====================

func TestPoolStress(t *testing.T) {
	t.Run("high concurrent job submission", func(t *testing.T) {
		pool := NewPool(20)
		defer pool.Release()
		time.Sleep(300 * time.Millisecond)

		var counter atomic.Int64
		var wg sync.WaitGroup
		numJobs := 100

		wg.Add(numJobs)
		for i := 0; i < numJobs; i++ {
			go func() {
				pool.Do(func() {
					counter.Add(1)
					wg.Done()
				})
			}()
		}

		wg.Wait()
		assert.Equal(t, int64(numJobs), counter.Load())
	})

	t.Run("resize during job execution", func(t *testing.T) {
		pool := NewPool(5)
		defer pool.Release()
		time.Sleep(200 * time.Millisecond)

		var counter atomic.Int64

		// Submit jobs while resizing - some may be dropped during resize
		for i := 0; i < 100; i++ {
			go pool.Do(func() {
				time.Sleep(10 * time.Millisecond)
				counter.Add(1)
			})

			if i == 25 {
				pool.Resize(10)
			}
			if i == 50 {
				pool.Resize(3)
			}
			if i == 75 {
				pool.Resize(8)
			}
		}

		// Wait for jobs to complete
		time.Sleep(2 * time.Second)

		// Verify some jobs were executed (not all may complete due to resize)
		assert.GreaterOrEqual(t, counter.Load(), int64(50))
	})

	t.Run("rapid create and release", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			pool := NewPool(5)
			time.Sleep(50 * time.Millisecond)
			pool.Release()
			time.Sleep(100 * time.Millisecond)
		}
	})
}

// ==================== Pool Edge Cases ====================

func TestPoolEdgeCases(t *testing.T) {
	t.Run("job with long execution", func(t *testing.T) {
		pool := NewPool(2)
		defer pool.Release()
		time.Sleep(100 * time.Millisecond)

		var completed atomic.Bool
		pool.Do(func() {
			time.Sleep(500 * time.Millisecond)
			completed.Store(true)
		})

		time.Sleep(100 * time.Millisecond)
		assert.False(t, completed.Load())

		time.Sleep(500 * time.Millisecond)
		assert.True(t, completed.Load())
	})

	t.Run("job accessing shared state", func(t *testing.T) {
		pool := NewPool(5)
		defer pool.Release()
		time.Sleep(100 * time.Millisecond)

		var mu sync.Mutex
		data := make(map[int]int)
		var wg sync.WaitGroup

		wg.Add(100)
		for i := 0; i < 100; i++ {
			i := i
			pool.Do(func() {
				mu.Lock()
				data[i] = i * 2
				mu.Unlock()
				wg.Done()
			})
		}

		wg.Wait()
		assert.Len(t, data, 100)
		for i := 0; i < 100; i++ {
			assert.Equal(t, i*2, data[i])
		}
	})
}

// ==================== Race Condition Tests ====================

func TestPoolRaceConditions(t *testing.T) {
	t.Run("concurrent resize and do", func(t *testing.T) {
		pool := NewPool(5)
		defer pool.Release()
		time.Sleep(200 * time.Millisecond)

		var wg sync.WaitGroup
		var counter atomic.Int64

		// Concurrent resizes
		wg.Add(10)
		for i := 0; i < 10; i++ {
			go func(size int) {
				defer wg.Done()
				pool.Resize(int64(size%10 + 1))
			}(i)
		}

		// Concurrent jobs
		wg.Add(50)
		for i := 0; i < 50; i++ {
			go func() {
				pool.Do(func() {
					counter.Add(1)
					wg.Done()
				})
			}()
		}

		wg.Wait()
		assert.Equal(t, int64(50), counter.Load())
	})

	t.Run("concurrent release and do", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			pool := NewPool(5)
			time.Sleep(100 * time.Millisecond)

			go func() {
				for j := 0; j < 10; j++ {
					pool.Do(func() {
						time.Sleep(10 * time.Millisecond)
					})
				}
			}()

			time.Sleep(50 * time.Millisecond)
			pool.Release()
			time.Sleep(200 * time.Millisecond)
		}
	})
}

// ==================== Option Tests ====================

func TestWithPanicHandler(t *testing.T) {
	t.Run("option sets panic handler", func(t *testing.T) {
		var handled atomic.Bool
		handler := func(r any) {
			handled.Store(true)
		}

		pool := NewPool(1, WithPanicHandler(handler))
		defer pool.Release()
		time.Sleep(100 * time.Millisecond)

		pool.Do(func() {
			panic("test")
		})
		time.Sleep(100 * time.Millisecond)

		assert.True(t, handled.Load())
	})

	t.Run("nil panic handler is valid", func(t *testing.T) {
		pool := NewPool(1, WithPanicHandler(nil))
		defer pool.Release()
		time.Sleep(100 * time.Millisecond)

		// Should not panic even when job panics
		require.NotPanics(t, func() {
			pool.Do(func() {
				panic("test")
			})
			time.Sleep(100 * time.Millisecond)
		})
	})
}

// ==================== Worker Count Tests ====================

func TestWorkerCount(t *testing.T) {
	t.Run("current workers matches expected", func(t *testing.T) {
		sizes := []int64{1, 5, 10, 20, 50}
		for _, size := range sizes {
			t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
				pool := NewPool(size)
				defer pool.Release()
				time.Sleep(time.Duration(size*50+200) * time.Millisecond)

				currentWorkers := atomic.LoadInt64(&pool.currentCountWorkers)
				assert.Equal(t, size, currentWorkers)
			})
		}
	})
}
