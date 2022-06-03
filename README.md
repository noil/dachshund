<p align="center">
    <img width="757" src="https://github.com/noil/dachshund/blob/master/dh.png?raw=true">
</p>

## Introduction

Library `dachshund` implements a simple  workers pool.

## Features:

- Recycling the number of workers automatically
- Changing (increase/descrease) the number of workers on the fly, without restarting or stopping the service

### Installation

To install this package, you need to setup your Go workspace. The simplest way to install the library is to run:
```powershell
$ go get github.com/noil/dachshund
```

## How to use

Create function 
```go
func() {
	fmt.Println("Hello, World!")
}
```

## Example

```go
package main

import (
	"fmt"
	"github.com/noil/dachshund"
)

func main() {
	f := func() {
		fmt.Println("Hello, World!")
	}

	pool := NewPool(10, *log.Default())
	defer pool.Release()
	pool.Do(f)
	time.Sleep(1 * time.Second)
}
```

License
----

MIT