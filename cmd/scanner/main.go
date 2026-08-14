// Command scanner searches a core dump for the deterministic marker produced
// by cmd/subject without embedding that marker in either binary.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Saku0512/What-and-How-Much-Is-Removed-from-the-Go-1.26-Runtime-and-Secrets-/internal/marker"
)

const chunkSize = 4 << 20

type result struct {
	File        string  `json:"file"`
	Occurrences int     `json:"occurrences"`
	Offsets     []int64 `json:"first_offsets,omitempty"`
}

func main() {
	countOnly := flag.Bool("count-only", false, "print only the occurrence count")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: scanner [-count-only] CORE_FILE")
		os.Exit(2)
	}

	path := flag.Arg(0)
	f, err := os.Open(path)
	if err != nil {
		fatal(err)
	}
	defer f.Close()

	count, offsets, err := scan(f, marker.SearchPattern())
	if err != nil {
		fatal(err)
	}

	if *countOnly {
		fmt.Println(count)
		return
	}

	if err := json.NewEncoder(os.Stdout).Encode(result{
		File:        path,
		Occurrences: count,
		Offsets:     offsets,
	}); err != nil {
		fatal(err)
	}
}

func scan(r io.Reader, pattern []byte) (int, []int64, error) {
	buf := make([]byte, chunkSize)
	tail := make([]byte, 0, len(pattern)-1)
	var totalRead int64
	count := 0
	offsets := make([]int64, 0, 8)

	for {
		n, err := r.Read(buf)
		if n > 0 {
			window := make([]byte, len(tail)+n)
			copy(window, tail)
			copy(window[len(tail):], buf[:n])
			base := totalRead - int64(len(tail))

			for searchFrom := 0; ; {
				i := bytes.Index(window[searchFrom:], pattern)
				if i < 0 {
					break
				}
				absolute := base + int64(searchFrom+i)
				count++
				if len(offsets) < cap(offsets) {
					offsets = append(offsets, absolute)
				}
				searchFrom += i + 1
			}

			totalRead += int64(n)
			keep := len(pattern) - 1
			if keep > len(window) {
				keep = len(window)
			}
			tail = append(tail[:0], window[len(window)-keep:]...)
		}
		if err == io.EOF {
			return count, offsets, nil
		}
		if err != nil {
			return 0, nil, err
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
