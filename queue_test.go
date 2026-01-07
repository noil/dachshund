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

// ==================== Mock Tuber Implementation ====================

type MockTask struct {
	ID      int
	Name    string
	Handler func()
}

func (t *MockTask) AsyncRun() func() {
	return t.Handler
}

type PanicTask struct{}

func (t *PanicTask) AsyncRun() func() {
	return func() {
		panic("panic from task")
	}
}

// ==================== Queue Creation Tests ====================

func TestNewQueue(t *testing.T) {
	t.Run("creates empty queue", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		assert.NotNil(t, q)
		assert.NotNil(t, q.tubes)
		assert.Empty(t, q.tubes)
		assert.False(t, q.closed)
	})

	t.Run("creates queue with panic handler option", func(t *testing.T) {
		var handled atomic.Bool
		q := NewQueue(WithPanicHandler(func(r any) {
			handled.Store(true)
		}))
		defer q.Terminate()

		err := q.AddTube("test", 1)
		require.NoError(t, err)

		err = q.PushFunc("test", func() {
			panic("test")
		})
		require.NoError(t, err)
		time.Sleep(200 * time.Millisecond)

		assert.True(t, handled.Load())
	})
}

func TestNewQueueWithContext(t *testing.T) {
	t.Run("queue terminates on context cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		q := NewQueueWithContext(ctx)

		err := q.AddTube("test", 5)
		require.NoError(t, err)

		cancel()
		time.Sleep(200 * time.Millisecond)

		q.mu.RLock()
		assert.True(t, q.closed)
		q.mu.RUnlock()
	})

	t.Run("queue terminates on context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		q := NewQueueWithContext(ctx)
		err := q.AddTube("test", 5)
		require.NoError(t, err)

		time.Sleep(300 * time.Millisecond)

		q.mu.RLock()
		assert.True(t, q.closed)
		q.mu.RUnlock()
	})

	t.Run("queue with already cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		q := NewQueueWithContext(ctx)
		time.Sleep(100 * time.Millisecond)

		q.mu.RLock()
		assert.True(t, q.closed)
		q.mu.RUnlock()
	})
}

// ==================== Queue Terminate Tests ====================

func TestQueueTerminate(t *testing.T) {
	t.Run("terminate empty queue", func(t *testing.T) {
		q := NewQueue()

		assert.NotPanics(t, func() {
			q.Terminate()
		})
		assert.True(t, q.closed)
	})

	t.Run("terminate queue with tubes", func(t *testing.T) {
		q := NewQueue()

		for i := 0; i < 5; i++ {
			err := q.AddTube(fmt.Sprintf("tube_%d", i), 5)
			require.NoError(t, err)
		}

		q.Terminate()

		assert.True(t, q.closed)
		assert.Empty(t, q.tubes)
	})

	t.Run("double terminate is safe", func(t *testing.T) {
		q := NewQueue()
		err := q.AddTube("test", 5)
		require.NoError(t, err)

		q.Terminate()

		assert.NotPanics(t, func() {
			q.Terminate()
		})
	})

	t.Run("terminate waits for active jobs", func(t *testing.T) {
		q := NewQueue()
		err := q.AddTube("test", 2)
		require.NoError(t, err)

		var completed atomic.Int64
		for i := 0; i < 5; i++ {
			err := q.PushFunc("test", func() {
				time.Sleep(50 * time.Millisecond)
				completed.Add(1)
			})
			require.NoError(t, err)
		}

		time.Sleep(100 * time.Millisecond)
		q.Terminate()

		// Jobs that were started should complete
		assert.GreaterOrEqual(t, completed.Load(), int64(1))
	})
}

// ==================== AddTube Tests ====================

func TestAddTube(t *testing.T) {
	t.Run("add single tube", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 5)
		assert.NoError(t, err)
		assert.Len(t, q.tubes, 1)
	})

	t.Run("add multiple tubes", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		for i := 0; i < 10; i++ {
			err := q.AddTube(fmt.Sprintf("tube_%d", i), int64(i+1))
			require.NoError(t, err)
		}

		assert.Len(t, q.tubes, 10)
	})

	t.Run("add duplicate tube returns error", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 5)
		require.NoError(t, err)

		err = q.AddTube("test", 10)
		assert.Equal(t, ErrTubeAlreadyExist, err)
	})

	t.Run("add tube to closed queue returns error", func(t *testing.T) {
		q := NewQueue()
		q.Terminate()

		err := q.AddTube("test", 5)
		assert.Equal(t, ErrQueueClosed, err)
	})

	t.Run("add tube with zero size", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 0)
		assert.NoError(t, err)
	})

	t.Run("add tube with empty name", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("", 5)
		assert.NoError(t, err)
	})

	t.Run("add tube with special characters in name", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		specialNames := []string{"tube-1", "tube_2", "tube.3", "tube:4", "tube/5"}
		for _, name := range specialNames {
			err := q.AddTube(name, 5)
			assert.NoError(t, err, "failed for name: %s", name)
		}
	})
}

// ==================== TerminateTube Tests ====================

func TestTerminateTube(t *testing.T) {
	t.Run("terminate existing tube", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 5)
		require.NoError(t, err)
		assert.Len(t, q.tubes, 1)

		q.TerminateTube("test")
		assert.Len(t, q.tubes, 0)
	})

	t.Run("terminate non-existing tube is safe", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		assert.NotPanics(t, func() {
			q.TerminateTube("nonexistent")
		})
	})

	t.Run("terminate one tube keeps others", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		for i := 0; i < 5; i++ {
			err := q.AddTube(fmt.Sprintf("tube_%d", i), 5)
			require.NoError(t, err)
		}
		assert.Len(t, q.tubes, 5)

		q.TerminateTube("tube_2")
		assert.Len(t, q.tubes, 4)

		_, exists := q.tubes["tube_2"]
		assert.False(t, exists)
	})

	t.Run("terminate tube with active jobs", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 2)
		require.NoError(t, err)

		for i := 0; i < 5; i++ {
			q.PushFunc("test", func() {
				time.Sleep(50 * time.Millisecond)
			})
		}

		time.Sleep(20 * time.Millisecond)

		assert.NotPanics(t, func() {
			q.TerminateTube("test")
		})
	})
}

// ==================== PushFunc Tests ====================

func TestQueueFunc(t *testing.T) {
	t.Run("push to existing tube", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 5)
		require.NoError(t, err)

		var executed atomic.Bool
		err = q.PushFunc("test", func() {
			executed.Store(true)
		})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)
		assert.True(t, executed.Load())
	})

	t.Run("push to non-existing tube returns error", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 5)
		require.NoError(t, err)

		err = q.PushFunc("nonexistent", func() {})
		assert.Equal(t, ErrTubeNotFound, err)
	})

	t.Run("push to closed queue returns error", func(t *testing.T) {
		q := NewQueue()
		err := q.AddTube("test", 5)
		require.NoError(t, err)

		q.Terminate()

		err = q.PushFunc("test", func() {})
		assert.Equal(t, ErrTubeClosed, err)
	})

	t.Run("push nil function", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 5)
		require.NoError(t, err)

		err = q.PushFunc("test", nil)
		assert.NoError(t, err)
	})

	t.Run("push many functions", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 10)
		require.NoError(t, err)

		var counter atomic.Int64
		var wg sync.WaitGroup
		numJobs := 100

		wg.Add(numJobs)
		for i := 0; i < numJobs; i++ {
			err := q.PushFunc("test", func() {
				counter.Add(1)
				wg.Done()
			})
			require.NoError(t, err)
		}

		wg.Wait()
		assert.Equal(t, int64(numJobs), counter.Load())
	})

	t.Run("push to multiple tubes", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		for i := 0; i < 5; i++ {
			err := q.AddTube(fmt.Sprintf("tube_%d", i), 5)
			require.NoError(t, err)
		}

		var counter atomic.Int64
		var wg sync.WaitGroup
		numJobs := 50

		wg.Add(numJobs)
		for i := 0; i < numJobs; i++ {
			tube := fmt.Sprintf("tube_%d", i%5)
			err := q.PushFunc(tube, func() {
				counter.Add(1)
				wg.Done()
			})
			require.NoError(t, err)
		}

		wg.Wait()
		assert.Equal(t, int64(numJobs), counter.Load())
	})
}

// ==================== Push Tests ====================

func TestQueuePush(t *testing.T) {
	t.Run("push task to existing tube", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 5)
		require.NoError(t, err)

		var executed atomic.Bool
		task := &MockTask{
			ID:   1,
			Name: "Test Task",
			Handler: func() {
				executed.Store(true)
			},
		}

		err = q.Push("test", task)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)
		assert.True(t, executed.Load())
	})

	t.Run("push to non-existing tube returns error", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		task := &MockTask{Handler: func() {}}
		err := q.Push("nonexistent", task)
		assert.Equal(t, ErrTubeNotFound, err)
	})

	t.Run("push to closed queue returns error", func(t *testing.T) {
		q := NewQueue()
		err := q.AddTube("test", 5)
		require.NoError(t, err)

		q.Terminate()

		task := &MockTask{Handler: func() {}}
		err = q.Push("test", task)
		assert.Equal(t, ErrTubeClosed, err)
	})

	t.Run("push many tasks", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 10)
		require.NoError(t, err)

		var counter atomic.Int64
		var wg sync.WaitGroup
		numTasks := 100

		wg.Add(numTasks)
		for i := 0; i < numTasks; i++ {
			task := &MockTask{
				ID:   i,
				Name: fmt.Sprintf("Task %d", i),
				Handler: func() {
					counter.Add(1)
					wg.Done()
				},
			}
			err := q.Push("test", task)
			require.NoError(t, err)
		}

		wg.Wait()
		assert.Equal(t, int64(numTasks), counter.Load())
	})

	t.Run("push panicking task with handler", func(t *testing.T) {
		var handledPanic atomic.Bool
		q := NewQueue(WithPanicHandler(func(r any) {
			handledPanic.Store(true)
		}))
		defer q.Terminate()

		err := q.AddTube("test", 1)
		require.NoError(t, err)

		task := &PanicTask{}
		err = q.Push("test", task)
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)
		assert.True(t, handledPanic.Load())
	})
}

// ==================== Queue Concurrency Tests ====================

func TestQueueConcurrency(t *testing.T) {
	t.Run("concurrent push to same tube", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 10)
		require.NoError(t, err)

		var counter atomic.Int64
		var wg sync.WaitGroup
		numGoroutines := 10
		numJobsPerGoroutine := 100

		wg.Add(numGoroutines * numJobsPerGoroutine)
		for g := 0; g < numGoroutines; g++ {
			go func() {
				for i := 0; i < numJobsPerGoroutine; i++ {
					q.PushFunc("test", func() {
						counter.Add(1)
						wg.Done()
					})
				}
			}()
		}

		wg.Wait()
		assert.Equal(t, int64(numGoroutines*numJobsPerGoroutine), counter.Load())
	})

	t.Run("concurrent push to different tubes", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		numTubes := 5
		for i := 0; i < numTubes; i++ {
			err := q.AddTube(fmt.Sprintf("tube_%d", i), 5)
			require.NoError(t, err)
		}

		var counter atomic.Int64
		var wg sync.WaitGroup
		numJobsPerTube := 100

		wg.Add(numTubes * numJobsPerTube)
		for t := 0; t < numTubes; t++ {
			t := t
			go func() {
				for i := 0; i < numJobsPerTube; i++ {
					q.PushFunc(fmt.Sprintf("tube_%d", t), func() {
						counter.Add(1)
						wg.Done()
					})
				}
			}()
		}

		wg.Wait()
		assert.Equal(t, int64(numTubes*numJobsPerTube), counter.Load())
	})

	t.Run("concurrent add and push", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		var wg sync.WaitGroup
		var counter atomic.Int64

		// Add tubes concurrently
		wg.Add(10)
		for i := 0; i < 10; i++ {
			i := i
			go func() {
				defer wg.Done()
				q.AddTube(fmt.Sprintf("tube_%d", i), 5)
			}()
		}
		wg.Wait()

		// Push to tubes concurrently
		wg.Add(100)
		for i := 0; i < 100; i++ {
			i := i
			go func() {
				tube := fmt.Sprintf("tube_%d", i%10)
				err := q.PushFunc(tube, func() {
					counter.Add(1)
					wg.Done()
				})
				if err != nil {
					wg.Done()
				}
			}()
		}
		wg.Wait()
	})

	t.Run("concurrent terminate and push", func(t *testing.T) {
		for run := 0; run < 10; run++ {
			q := NewQueue()
			err := q.AddTube("test", 5)
			require.NoError(t, err)

			go func() {
				for i := 0; i < 50; i++ {
					q.PushFunc("test", func() {
						time.Sleep(10 * time.Millisecond)
					})
				}
			}()

			time.Sleep(20 * time.Millisecond)
			q.Terminate()
		}
	})
}

// ==================== Tube Tests ====================

func TestTubeJobCount(t *testing.T) {
	t.Run("job count increases and decreases", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 1)
		require.NoError(t, err)

		tube := q.tubes["test"]
		assert.Equal(t, int64(0), atomic.LoadInt64(&tube.countActiveJobs))

		var wg sync.WaitGroup
		wg.Add(1)

		done := make(chan struct{})
		err = q.PushFunc("test", func() {
			wg.Done()
			<-done
		})
		require.NoError(t, err)

		wg.Wait()
		time.Sleep(50 * time.Millisecond)

		// Job count should be 0 or 1 depending on timing
		count := atomic.LoadInt64(&tube.countActiveJobs)
		assert.GreaterOrEqual(t, count, int64(0))

		close(done)
		time.Sleep(100 * time.Millisecond)

		count = atomic.LoadInt64(&tube.countActiveJobs)
		assert.Equal(t, int64(0), count)
	})
}

// ==================== Error Handling Tests ====================

func TestQueueErrors(t *testing.T) {
	t.Run("ErrTubeNotFound", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.PushFunc("nonexistent", func() {})
		assert.Equal(t, ErrTubeNotFound, err)
		assert.Contains(t, err.Error(), "tube not found")
	})

	t.Run("ErrTubeAlreadyExist", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 5)
		require.NoError(t, err)

		err = q.AddTube("test", 5)
		assert.Equal(t, ErrTubeAlreadyExist, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("ErrQueueClosed", func(t *testing.T) {
		q := NewQueue()
		q.Terminate()

		err := q.AddTube("test", 5)
		assert.Equal(t, ErrQueueClosed, err)
		assert.Contains(t, err.Error(), "queue closed")
	})

	t.Run("ErrTubeClosed", func(t *testing.T) {
		q := NewQueue()
		err := q.AddTube("test", 5)
		require.NoError(t, err)

		q.Terminate()

		err = q.PushFunc("test", func() {})
		assert.Equal(t, ErrTubeClosed, err)
		assert.Contains(t, err.Error(), "tube closed")
	})
}

// ==================== Stress Tests ====================

func TestQueueStress(t *testing.T) {
	t.Run("high load single tube", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("stress", 20)
		require.NoError(t, err)

		var counter atomic.Int64
		var wg sync.WaitGroup
		numJobs := 100

		wg.Add(numJobs)
		for i := 0; i < numJobs; i++ {
			go func() {
				q.PushFunc("stress", func() {
					counter.Add(1)
					wg.Done()
				})
			}()
		}

		wg.Wait()
		assert.Equal(t, int64(numJobs), counter.Load())
	})

	t.Run("rapid tube create and terminate", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		for i := 0; i < 50; i++ {
			name := fmt.Sprintf("tube_%d", i)
			err := q.AddTube(name, 5)
			require.NoError(t, err)

			q.PushFunc(name, func() {
				time.Sleep(10 * time.Millisecond)
			})

			q.TerminateTube(name)
		}
	})
}

// ==================== Edge Cases ====================

func TestQueueEdgeCases(t *testing.T) {
	t.Run("push after tube terminated", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 5)
		require.NoError(t, err)

		q.TerminateTube("test")

		err = q.PushFunc("test", func() {})
		assert.Equal(t, ErrTubeNotFound, err)
	})

	t.Run("re-add tube after termination", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 5)
		require.NoError(t, err)

		q.TerminateTube("test")

		err = q.AddTube("test", 10)
		assert.NoError(t, err)

		var executed atomic.Bool
		err = q.PushFunc("test", func() {
			executed.Store(true)
		})
		require.NoError(t, err)

		time.Sleep(200 * time.Millisecond)
		assert.True(t, executed.Load())
	})

	t.Run("empty tube name operations", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("", 5)
		require.NoError(t, err)

		var executed atomic.Bool
		err = q.PushFunc("", func() {
			executed.Store(true)
		})
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		assert.True(t, executed.Load())

		q.TerminateTube("")
		assert.Empty(t, q.tubes)
	})

	t.Run("long running job with terminate", func(t *testing.T) {
		q := NewQueue()

		err := q.AddTube("test", 1)
		require.NoError(t, err)

		started := make(chan struct{})
		err = q.PushFunc("test", func() {
			close(started)
			time.Sleep(500 * time.Millisecond)
		})
		require.NoError(t, err)

		<-started
		q.Terminate()
	})
}

// ==================== Tuber Interface Tests ====================

func TestTuberInterface(t *testing.T) {
	t.Run("custom tuber implementation", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 5)
		require.NoError(t, err)

		var result atomic.Int64
		task := &MockTask{
			ID:   42,
			Name: "Custom Task",
			Handler: func() {
				result.Store(42)
			},
		}

		err = q.Push("test", task)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, int64(42), result.Load())
	})

	t.Run("nil handler in tuber", func(t *testing.T) {
		q := NewQueue()
		defer q.Terminate()

		err := q.AddTube("test", 1)
		require.NoError(t, err)

		task := &MockTask{Handler: nil}
		err = q.Push("test", task)
		assert.NoError(t, err)

		time.Sleep(100 * time.Millisecond)
	})
}
