package main

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/noil/dachshund"
)

var wg sync.WaitGroup

type Demo struct {
	count int32
}

func (d *Demo) Do(data interface{}) {
	if _, ok := data.(int); ok {
		atomic.AddInt32(&d.count, 1)
	}
	wg.Done()
}

func main() {
	demo := &Demo{}
	pool := dachshund.NewPool(10, demo)
	defer func() {
		err := pool.Release()
		if err != nil {
			fmt.Println(err)
		}
	}()
	for i := 0; i < 1000000; i++ {
		wg.Add(1)
		pool.Do(i)
		wg.Wait()
	}

	fmt.Println(demo.count)
}
