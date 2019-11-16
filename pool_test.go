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

type Task3 struct{}

func (s *Task3) Do(data interface{}) {
	fmt.Println("task 3 has recieved data")
	time.Sleep(time.Duration(10) * time.Millisecond)
}

type Task4 struct{}

func (s *Task4) Do(data interface{}) {
	fmt.Println("task 4 has recieved data")
	time.Sleep(time.Duration(20) * time.Millisecond)
}

func TestNewPool(t *testing.T) {
	task := &Task1{result: make(chan interface{}, 20)}
	p := NewPool(10, 20, task)
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

func TestReload(t *testing.T) {
	task := &Task3{}
	p := NewPool(1, 2, task)
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

func TestReloadTask(t *testing.T) {
	task3 := &Task3{}
	task4 := &Task4{}
	p := NewPool(10, 20, task3)
	defer p.Release()
	go func() {
		select {
		case <-time.After(500 * time.Millisecond):
			err := p.Reload(10, 20, task4)
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
