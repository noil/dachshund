package dachshund

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPool(t *testing.T) {
	pool := NewPool(10, nil)
	time.Sleep(1 * time.Second)
	assert.Equal(t, int64(10), atomic.LoadInt64(&pool.currentCountWorkers), "they should by equal")

	pool.Resize(20)
	time.Sleep(1 * time.Second)
	assert.Equal(t, int64(20), atomic.LoadInt64(&pool.currentCountWorkers), "they should by equal")

	pool.Resize(10)
	time.Sleep(1 * time.Second)
	assert.Equal(t, int64(10), atomic.LoadInt64(&pool.currentCountWorkers), "they should by equal")

	a := 10
	pool.Do(func() {
		a = 20
	})
	time.Sleep(1 * time.Second)
	assert.Equal(t, 20, a, "they should by equal")
}
