package dachshund

import (
	"testing"
	"time"
)

const (
	RunTimes   = 1000
	BenchParam = 1
)

func BenchmarkConcurrent(b *testing.B) {
	p := NewPool(100, nil)
	defer p.Release()
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < RunTimes; i++ {
			p.Do(func() {
				time.Sleep(time.Duration(BenchParam) * time.Millisecond)
			})
			i++
		}
	}
	b.StopTimer()
}
