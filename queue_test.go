package dachshund

import (
	"fmt"
	"log"
	"testing"
	"time"
)

type Person struct {
	ID   int64
	Name string
}

func (p *Person) Intro() {
	fmt.Println(p.ID, p.Name)
}

func TestTube(t *testing.T) {
	queue := NewQueue(10, *log.Default())
	p := &Person{ID: 1, Name: "John Smith"}
	queue.PushFunc("fetch", func() {
		p.Intro()
	})
	time.Sleep(1 * time.Second)
}
