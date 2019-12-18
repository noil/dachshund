package dachshund

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type BTask1 struct {
	result chan interface{}
}

func (s *BTask1) Do(data interface{}) {
	switch v := data.(type) {
	case int:
		s.result <- v * v
	case int32:
		s.result <- v * v
	case int64:
		s.result <- v * v
	case float32:
		s.result <- v * v
	case float64:
		s.result <- v * v
	case string:
		s.result <- strings.ToUpper(v)
	default:
		s.result <- nil
	}
}

type BTask2 struct {
	result chan interface{}
}

func (s *BTask2) Do(data interface{}) {
	switch v := data.(type) {
	case int:
		s.result <- v * 2
	case int32:
		s.result <- v * 2
	case int64:
		s.result <- v * 2
	case float32:
		s.result <- v * 2
	case float64:
		s.result <- v * 2
	case string:
		s.result <- strings.Repeat(v, 2)
	default:
		s.result <- nil
	}
}

type BTask3 struct{}

func (s *BTask3) Do(data interface{}) {
	// fmt.Println("buffered task 3 has recieved data")
	time.Sleep(time.Duration(10) * time.Millisecond)
}

func TestNewBufferedPool(t *testing.T) {
	task := &BTask1{result: make(chan interface{}, 20)}
	p := NewBufferedPool("test2", 10, 20, task.Do, nil)
	defer func() {
		p.Release()
		fmt.Println("test new buffered pool has finished")
	}()
	i := 2
	p.Do(i)
	result := <-task.result
	if value, ok := result.(int); !ok || value != 4 {
		t.Error(
			"For", i,
			"expected", 4,
			"got", value,
		)
	}
	s := "Hello World!"
	p.Do(s)
	result = <-task.result
	if value, ok := result.(string); !ok || value != "HELLO WORLD!" {
		t.Error(
			"For", s,
			"expected", "HELLO WORLD!",
			"got", value,
		)
	}
}

type PanicTaskBuffered struct{}

func (s *PanicTaskBuffered) Do(data interface{}) {
	if value, ok := data.(string); ok {
		fmt.Println(value)
	} else {
		panic("invalid type")
	}
}

func TestPanicBuffered(t *testing.T) {
	pt := &PanicTaskBuffered{}
	p := NewBufferedPool("test3", 10, 20, pt.Do, nil)
	defer func() {
		p.Release()
		fmt.Println("test panic buffered has finished")
	}()

	for i := 0; i < 200; i++ {
		p.Do(i)
	}
	p.Do("buffered. i was born")
}
