package builtins

import (
	"context"
	"fmt"
	"log"
	"runtime"
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

type Panic struct {
	Recovered interface{}
}

func (p Panic) Error() string {
	return fmt.Sprintf("panic: %s", p.Recovered)
}

func Recover() error {
	err := recover()
	if err != nil {
		return Panic{err}
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

func Parallel(ctx context.Context, fns ...func(context.Context) error) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan error, len(fns))
	for _, fn := range fns {
		fn := fn
		Go(func() {
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
		})
	}
	for _ = range fns {
		err = <-ch
		if err != nil {
			return
		}
	}

	return
}
