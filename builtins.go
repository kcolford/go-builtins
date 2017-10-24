package builtins

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

func Go(fn func()) {
	go func() {
		defer func() { Ignore(Recover()) }()
		fn()
	}()
}

func Ignore(err error) {
	if err != nil {
		log.Printf("ignored: %s", err)
	}
}

type PanicError struct {
	Recovered interface{}
}

func (p PanicError) Error() string {
	return fmt.Sprintf("panic: %s", p.Recovered)
}

func Recover() error {
	err := recover()
	if err != nil {
		return PanicError{err}
	}
	return nil
}

func End() {
	log.Print("Stopped")

	// keep the console open on windows machines
	if runtime.GOOS == "windows" {
		for {
			time.Sleep(100000 * time.Hour)
		}
	}
}

func Panic(val interface{}) {
	panic(val)
}

func Parallel(ctx context.Context, fns ...func(context.Context) error) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := make(chan error, len(fns))
	for _, fn := range fns {
		fn := fn
		go func() {
			ch <- func() (e error) {
				defer func() {
					err := Recover()
					if err != nil {
						e = err
					}
				}()
				e = fn(ctx)
				return
			}()
		}()
	}
	for _ = range fns {
		err = <-ch
		if err != nil {
			if p, ok := err.(PanicError); ok {
				Panic(p.Recovered)
			}
			return
		}
	}

	return
}

type listener struct {
	sync.WaitGroup
	ch chan<- interface{}
}

type Broadcaster struct {
	sync.RWMutex
	listeners []listener
}

func (b *Broadcaster) Broadcast() chan<- interface{} {
	ch := make(chan interface{})
	go func() {
		for c := range ch {
			c := c
			b.RLock()
			defer b.RUnlock()
			for _, listener := range b.listeners {
				listener := listener
				listener.Add(1)
				go func() {
					defer listener.Done()
					listener.ch <- c
				}()
			}
		}
	}()
	return ch
}

func (b *Broadcaster) Listen() <-chan interface{} {
	b.Lock()
	defer b.Unlock()
	ch := make(chan interface{})
	b.listeners = append(b.listeners, listener{ch: ch})
	return ch
}

func (b *Broadcaster) Close() {
	b.Lock()
	defer b.Unlock()
	for _, listener := range b.listeners {
		listener := listener
		go func() {
			defer close(listener.ch)
			listener.Wait()
		}()
	}
	b.listeners = nil
}
