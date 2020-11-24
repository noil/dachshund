package dachshund

import (
	"fmt"
	"testing"
	"time"
)

func TestNewFuncPool(t *testing.T) {
	pool := NewFuncPool("test", 10, nil)
	pool.Do(func() {
		fmt.Println("hello world!")
	})
	time.Sleep(2 * time.Second)
}
