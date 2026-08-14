package main

import (
	"bytes"
	"testing"
)

func TestScanFindsPatternsAcrossReadBoundaries(t *testing.T) {
	pattern := []byte("needle")
	input := append(bytes.Repeat([]byte{'x'}, chunkSize-3), pattern...)
	input = append(input, bytes.Repeat([]byte{'y'}, 17)...)
	input = append(input, pattern...)

	count, offsets, err := scan(bytes.NewReader(input), pattern)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if len(offsets) != 2 {
		t.Fatalf("len(offsets) = %d, want 2", len(offsets))
	}
}
