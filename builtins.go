package builtins

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
)

// Start a goroutine while ignoring any panics.
func Go(fn func()) {
	go func() {
		defer func() { Ignore(Recover()) }()
		fn()
	}()
}

// Ignore an error. This implementation logs it.
func Ignore(err error) {
	if err != nil {
		log.Printf("ignored: %s", err)
	}
}

// An error caused by a panic.
type PanicError struct {
	// The value that was passed to panic.
	Recovered interface{}
}

func (p *PanicError) Error() string {
	return fmt.Sprintf("panic: %s", p.Recovered)
}

// Recover from a panic, producing an error.
func Recover() *PanicError {
	err := recover()
	if err != nil {
		return &PanicError{err}
	}
	return nil
}

// End a command line application.
func End() {
	log.Print("Stopped")

	// keep the console open on windows machines
	if runtime.GOOS == "windows" {
		for {
			time.Sleep(100000 * time.Hour)
		}
	}
}

type thrown struct {
	val interface{}
}

func (t thrown) Error() string {
	return fmt.Sprint(t.val)
}

// Throw an exception (using panic).
func Throw(val interface{}) {
	panic(thrown{val})
}

// Catch a thrown error. Call this from within a deferred function.
func Catch() (err error) {
	p := recover()
	if p != nil {
		if t, ok := p.(thrown); ok {
			err = t
		} else {
			panic(p)
		}
	}
	return
}

// Run all `fns` in parallel and wait for them to complete, immediatly
// returning the first error encountered or pann
func Parallel(ctx context.Context, fns ...func(context.Context) error) (err error) {
	// cancel all children when this function returns (such as prematurely by
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// error channel and panic channel, same length as number of
	// goroutines so that they won't block and won't leak
	ch := make(chan error, len(fns))
	pnc := make(chan interface{}, len(fns))
	for _, fn := range fns {
		fn := fn
		go func() {
			defer func() {
				err := recover()
				if err != nil {
					pnc <- err
				}
			}()
			ch <- fn(ctx)
		}()
	}
	for _ = range fns {
		select {
		case err = <-ch:
			if err != nil {
				return
			}
		case p := <-pnc:
			panic(p)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return
}

type listener struct {
	sync.WaitGroup
	ch chan<- interface{}
}

// A broadcasting channel. Use Broadcast() to get a channel for
// broadcasting, Listen() to get a channel for listening to more
// broadcasts, and Close() for closing all listening channels. This is
// completely thread safe to use just like channels.
//
// Broadcasts are completely reliable, they are guaranteed to never be
// dropped until emptied or closed.
type Broadcaster struct {
	sync.RWMutex
	listeners []listener
}

// Empty all the listener channels to prevent a leak.
func (b *Broadcaster) Empty() {
	b.RLock()
	defer b.RUnlock()
	b.empty()
}

func (b *Broadcaster) empty() {
	for _, listener := range b.listeners {
		for {
			select {
			case <-listener.ch:
			default:
				break
			}
		}
	}
}

// Return a channel for sending broadcasts to all listeners. This
// channel must be closed or else it will leak.
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

// Return a channel for listening to broadcasts with.
func (b *Broadcaster) Listen() <-chan interface{} {
	b.Lock()
	defer b.Unlock()
	ch := make(chan interface{})
	b.listeners = append(b.listeners, listener{ch: ch})
	return ch
}

// Close all channels produced by Listen()
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
	b.empty()
	b.listeners = nil
}
