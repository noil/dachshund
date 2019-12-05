<p align="center">
    <img width="757" src="https://github.com/noil/dachshund/blob/master/dh.png?raw=true">
</p>

## Introduction

Library `dachshund` implements a simple  workers pool. You could use it as buffered pool for recieving new tasks or as classic pool without buffering.

## Features:

- Recycling the number of workers automatically
- Changing (increase/descrease) the number of workers on the fly, without restarting or stopping the service
- Changing (increase/descrease) queue buffer size on the fly, without restarting or stopping the service. If use buffered pool
- Changing method which pool calls on the fly

### Installation

To install this package, you need to setup your Go workspace. The simplest way to install the library is to run:
```powershell
$ go get github.com/noil/dachshund
```

## How to use

Worker must implements `Pooler` interface
```go
type Pooler interface {
	Do(data interface{})
}
```

## Example

```go
package main

import (
	"fmt"
	"github.com/noil/dachshund"
)

var wg sync.WaitGroup

type Foo struct{}

func (f *Foo) Do(data interface{}) {
    time.Sleep(time.Duration(10) * time.Millisecond)
    wg.Done()
}

func main() {
	f := &Foo{}
	bp := dachshund.NewBufferedPool(5, 10, f)
	defer bp.Release()
	for i := 0; i < 1000000; i++ {
		wg.Add(1)
		bp.Do(i)
		wg.Wait()
	}

	p := dachshund.NewPool(5, f)
	defer p.Release()
	for i := 0; i < 1000000; i++ {
		wg.Add(1)
		p.Do(i)
		wg.Wait()
	}
}
```

License
----

MIT