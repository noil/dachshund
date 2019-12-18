package main

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/noil/dachshund"
)

const SIZE = 1000000

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
	pool := dachshund.NewPool("demo", 10, demo.Do, nil)
	// ctx, cancel := context.WithCancel(context.Background())
	// pool := dachshund.NewPoolWithContext(ctx, 10, demo)

	defer pool.Release()
	// defer func() {
	// 	cancel()
	// 	time.Sleep(time.Second * 2)
	// }()
	wg.Add(SIZE)
	for i := 0; i < SIZE; i++ {
		pool.Do(i)
	}
	wg.Wait()

	fmt.Println(demo.count)
}
