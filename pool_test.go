package dachshund

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type Task1 struct {
	result chan interface{}
}

func (s *Task1) Do(data interface{}) {
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

type Task2 struct {
	result chan interface{}
}

func (s *Task2) Do(data interface{}) {
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

type Task3 struct {
}

func (s *Task3) Do(data interface{}) {
	// fmt.Println("task 3 has recieved data")
	time.Sleep(time.Duration(10) * time.Millisecond)
}

func TestNewPool(t *testing.T) {
	fmt.Println("test new pool has launched")
	task := &Task1{result: make(chan interface{}, 20)}
	p := NewPool("test1", 10, task.Do, nil)
	defer func() {
		p.Release()
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
	// time.Sleep(1 * time.Second)

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
	// time.Sleep(1 * time.Second)
}

func TestReload(t *testing.T) {
	fmt.Println("test reload has launched")
	task3 := &Task3{}
	p := NewPool("test2", 1, task3.Do, nil)
	defer p.Release()

	go func() {
		for i := 1; i < 5; i++ {
			<-time.After(300 * time.Millisecond)
			p.Reload(i)
			i++
		}
	}()
	for i := 0; i <= 100; i++ {
		p.Do(i)
		time.Sleep(20 * time.Millisecond)
	}
}

type PanicTask struct{}

func (s *PanicTask) Do(data interface{}) {
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		fmt.Println(r)
	// 	}
	// }()
	if value, ok := data.(string); ok {
		fmt.Println(value)
	} else {
		panic("invalid type")
	}
}

func TestPanic(t *testing.T) {
	pt := &PanicTask{}
	p := NewPool("test3", 10, pt.Do, nil)
	defer p.Release()

	for i := 0; i < 20; i++ {
		p.Do(i)
	}
	p.Do("i was born")
}
