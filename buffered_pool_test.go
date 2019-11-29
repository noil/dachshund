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
	fmt.Println("buffered task 3 has recieved data")
	time.Sleep(time.Duration(10) * time.Millisecond)
}

type BTask4 struct{}

func (s *BTask4) Do(data interface{}) {
	fmt.Println("buffered task 4 has recieved data")
	time.Sleep(time.Duration(20) * time.Millisecond)
}

func TestNewBufferedPool(t *testing.T) {
	task := &BTask1{result: make(chan interface{}, 20)}
	p := NewBufferedPool(10, 20, task)
	defer p.Release()
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

func TestReloadBuffered(t *testing.T) {
	task := &BTask3{}
	p := NewBufferedPool(1, 2, task)
	defer p.Release()
	go func() {
		i := 1
		for {
			select {
			case <-time.After(300 * time.Millisecond):
				if i > 5 {
					break
				}
				err := p.Reload(i, i*2, task)
				if err != nil {
					fmt.Println(err)
				}
				i++
			}
		}
	}()
	for i := 0; i <= 100; i++ {
		p.Do(i)
		time.Sleep(20 * time.Millisecond)
	}
}

func TestReloadTaskBuffered(t *testing.T) {
	p := NewBufferedPool(10, 20, &BTask3{})
	defer p.Release()
	go func() {
		select {
		case <-time.After(500 * time.Millisecond):
			err := p.Reload(10, 20, &BTask4{})
			if err != nil {
				fmt.Println(err)
			}
		}
	}()
	for i := 0; i <= 50; i++ {
		p.Do(i)
		time.Sleep(20 * time.Millisecond)
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
	p := NewBufferedPool(10, 20, &PanicTaskBuffered{})
	defer func() {
		err := p.Release()
		if err != nil {
			fmt.Println(err)
		}
	}()

	for i := 0; i < 200; i++ {
		p.Do(i)
	}
	p.Do("buffered. i was born")
}
