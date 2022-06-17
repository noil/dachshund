package dachshund

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Person struct {
	ID      int
	Name    string
	Handler func()
}

func (p *Person) AsyncRun() func() {
	return p.Handler
}

func TestQueueFunc(t *testing.T) {
	cases := []struct {
		Name        string
		CountTubes  int
		ActiveTube  string
		Func        func()
		ExpectedErr error
	}{
		{
			Name:        "Success",
			CountTubes:  10,
			ActiveTube:  "tube_2",
			Func:        func() {},
			ExpectedErr: nil,
		},
		{
			Name:        "Bad",
			CountTubes:  10,
			ActiveTube:  "20",
			Func:        func() {},
			ExpectedErr: ErrTubeNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			q := NewQueue()
			for i := 0; i < test.CountTubes; i++ {
				q.AddTube(fmt.Sprintf(`tube_%d`, i), 10)
			}
			err := q.PushFunc(test.ActiveTube, test.Func)
			assert.Equal(t, test.ExpectedErr, err)
		})
	}
}

func TestQueuePush(t *testing.T) {
	cases := []struct {
		Name        string
		CountTubes  int
		ActiveTube  string
		Person      *Person
		ExpectedErr error
	}{
		{
			Name:       "Success",
			CountTubes: 10,
			ActiveTube: "tube_2",
			Person: &Person{
				ID:   1,
				Name: "John Smith",
				Handler: func() {

				},
			},
			ExpectedErr: nil,
		},
		{
			Name:       "Bad",
			CountTubes: 10,
			ActiveTube: "20",
			Person: &Person{
				ID:   1,
				Name: "John Smith",
				Handler: func() {

				},
			},
			ExpectedErr: ErrTubeNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			q := NewQueue()
			for i := 0; i < test.CountTubes; i++ {
				q.AddTube(fmt.Sprintf(`tube_%d`, i), 10)
			}
			err := q.Push(test.ActiveTube, test.Person)
			assert.Equal(t, test.ExpectedErr, err)
		})
	}
}
