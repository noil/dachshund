package dachshund

import (
	"testing"
	"time"
)

const (
	BuffRunTimes   = 1000000
	BuffBenchParam = 10
)

type BuffDemo struct {
	r int
}

func (s *BuffDemo) Do(data interface{}) {
	time.Sleep(time.Duration(BuffBenchParam) * time.Millisecond)
}

func BenchmarkBufferedConcurrent(b *testing.B) {
	demo := &Demo{}
	p := NewBufferedPool(100, 2000, demo.Do, nil)
	defer p.Release()
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < BuffRunTimes; i++ {
			p.Do(i)
			i++
		}
	}
	b.StopTimer()
}
