package dachshund

import (
	"testing"
	"time"
)

const (
	RunTimes   = 1000000
	BenchParam = 10
)

type Demo struct {
	r int
}

func (s *Demo) Do(data interface{}) {
	time.Sleep(time.Duration(BenchParam) * time.Millisecond)
}

func BenchmarkConcurrent(b *testing.B) {
	p := NewPool(100, 2000, &Demo{})
	defer p.Release()
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < RunTimes; i++ {
			p.Do(i)
			i++
		}
	}
	b.StopTimer()
}
