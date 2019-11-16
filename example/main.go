package main

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/noil/dachshund"
	"github.com/spf13/viper"
)

var wg sync.WaitGroup

type Demo struct {
	count int32
}

func (d *Demo) Do(data interface{}) {
	if _, ok := data.(int); ok {
		atomic.AddInt32(&d.count, 1)
	}
	wg.Done()
}

func main() {
	viper.SetConfigType("yaml")
	var yamlExample = []byte(`
numOfWorkers: 10
queueBuffSize: 20
`)
	viper.ReadConfig(bytes.NewBuffer(yamlExample))

	d := &Demo{}
	pool := dachshund.NewPool(viper.GetInt("numOfWorkers"), viper.GetInt("queueBuffSize"), d)
	defer pool.Release()

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		pool.Reload(viper.GetInt("numOfWorkers"), viper.GetInt("queueBuffSize"), d)
	})

	for i := 0; i < 1000000; i++ {
		wg.Add(1)
		pool.Do(i)
		wg.Wait()
	}

	fmt.Println(d.count)
}
