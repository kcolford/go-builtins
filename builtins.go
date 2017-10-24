package builtins

import (
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

func Recover() error {
	err := recover()
	if err != nil {
		return fmt.Errorf("panic: %s", err)
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
