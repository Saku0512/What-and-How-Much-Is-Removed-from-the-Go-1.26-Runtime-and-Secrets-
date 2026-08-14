//go:build goexperiment.runtimesecret

// Command subject creates secret-like byte sequences in different storage and
// lifetime scenarios, then remains alive so another process can dump its memory.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/secret"
	"sort"
	"syscall"

	"github.com/Saku0512/What-and-How-Much-Is-Removed-from-the-Go-1.26-Runtime-and-Secrets-/internal/marker"
)

var (
	hold        = make(chan struct{})
	sink        uint64
	escaped     []byte
	globalBytes [marker.BufferSize]byte
	panicValue  = errors.New("intentional panic without secret data")
)

var cases = map[string]func(chan<- struct{}){
	"plain-stack-live":      plainStackLive,
	"plain-stack-returned":  plainStackReturned,
	"secret-stack-live":     secretStackLive,
	"secret-stack-returned": secretStackReturned,
	"plain-heap-after-gc":   plainHeapAfterGC,
	"secret-heap-live":      secretHeapLive,
	"secret-heap-before-gc": secretHeapBeforeGC,
	"secret-heap-after-gc":  secretHeapAfterGC,
	"secret-heap-escaped":   secretHeapEscaped,
	"secret-copy-out":       secretCopyOut,
	"secret-global":         secretGlobal,
	"secret-panic":          secretPanic,
}

func main() {
	mode := flag.String("mode", "", "experiment case to run")
	list := flag.Bool("list", false, "list experiment cases")
	flag.Parse()

	if *list {
		names := make([]string, 0, len(cases))
		for name := range cases {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Println(name)
		}
		return
	}

	run, ok := cases[*mode]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown case %q; use -list to list cases\n", *mode)
		os.Exit(2)
	}

	ready := make(chan struct{})
	go run(ready)
	<-ready

	// Only signal readiness after the worker has reached the precise lifetime
	// boundary selected by the case. The marker is never printed.
	fmt.Println("READY")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
}

//go:noinline
func consume(b []byte) {
	var v uint64 = 1469598103934665603
	for _, x := range b {
		v ^= uint64(x)
		v *= 1099511628211
	}
	sink = v
	runtime.KeepAlive(b)
}

//go:noinline
func makePlainStack() {
	var b [marker.BufferSize]byte
	marker.Fill(b[:])
	consume(b[:])
}

func plainStackLive(ready chan<- struct{}) {
	var b [marker.BufferSize]byte
	marker.Fill(b[:])
	consume(b[:])
	close(ready)
	<-hold
	runtime.KeepAlive(&b)
}

func plainStackReturned(ready chan<- struct{}) {
	makePlainStack()
	close(ready)
	<-hold
}

func secretStackLive(ready chan<- struct{}) {
	secret.Do(func() {
		var b [marker.BufferSize]byte
		marker.Fill(b[:])
		consume(b[:])
		close(ready)
		<-hold
		runtime.KeepAlive(&b)
	})
}

func secretStackReturned(ready chan<- struct{}) {
	secret.Do(func() {
		var b [marker.BufferSize]byte
		marker.Fill(b[:])
		consume(b[:])
	})
	close(ready)
	<-hold
}

//go:noinline
func makePlainHeap() {
	b := make([]byte, marker.BufferSize)
	marker.Fill(b)
	consume(b)
}

func plainHeapAfterGC(ready chan<- struct{}) {
	makePlainHeap()
	runtime.GC()
	close(ready)
	<-hold
}

func secretHeapLive(ready chan<- struct{}) {
	secret.Do(func() {
		b := make([]byte, marker.BufferSize)
		marker.Fill(b)
		consume(b)
		close(ready)
		<-hold
		runtime.KeepAlive(b)
	})
}

func secretHeapBeforeGC(ready chan<- struct{}) {
	secret.Do(func() {
		b := make([]byte, marker.BufferSize)
		marker.Fill(b)
		consume(b)
	})
	close(ready)
	<-hold
}

func secretHeapAfterGC(ready chan<- struct{}) {
	secret.Do(func() {
		b := make([]byte, marker.BufferSize)
		marker.Fill(b)
		consume(b)
	})
	runtime.GC()
	close(ready)
	<-hold
}

func secretHeapEscaped(ready chan<- struct{}) {
	secret.Do(func() {
		b := make([]byte, marker.BufferSize)
		marker.Fill(b)
		consume(b)
		escaped = b
	})
	runtime.GC()
	close(ready)
	<-hold
	runtime.KeepAlive(escaped)
}

func secretCopyOut(ready chan<- struct{}) {
	outside := make([]byte, marker.BufferSize)
	secret.Do(func() {
		var temporary [marker.BufferSize]byte
		marker.Fill(temporary[:])
		consume(temporary[:])
		copy(outside, temporary[:])
	})
	escaped = outside
	runtime.GC()
	close(ready)
	<-hold
	runtime.KeepAlive(escaped)
}

func secretGlobal(ready chan<- struct{}) {
	secret.Do(func() {
		marker.Fill(globalBytes[:])
		consume(globalBytes[:])
	})
	runtime.GC()
	close(ready)
	<-hold
	runtime.KeepAlive(&globalBytes)
}

func secretPanic(ready chan<- struct{}) {
	func() {
		defer func() {
			if recover() == nil {
				panic("secret.Do did not propagate the panic")
			}
		}()
		secret.Do(func() {
			var b [marker.BufferSize]byte
			marker.Fill(b[:])
			consume(b[:])
			panic(panicValue)
		})
	}()
	close(ready)
	<-hold
}
