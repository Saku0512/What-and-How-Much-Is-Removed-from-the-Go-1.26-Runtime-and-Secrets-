// Package marker generates a deterministic byte sequence used as a stand-in
// for secret material. The sequence itself is never embedded in the binaries.
package marker

const (
	// SearchSize is the number of consecutive bytes the scanner searches for.
	SearchSize = 256

	// BufferSize is deliberately larger than SearchSize. This makes the
	// returned-stack control less likely to be completely overwritten by the
	// few calls needed to put the process into its observation state.
	BufferSize = 4096

	seed uint64 = 0x6a09e667f3bcc909
)

// Fill writes a deterministic, high-entropy-looking sequence to dst.
//
// The generator is intentionally simple: this is a searchable marker, not a
// cryptographic random-number generator.
//
//go:noinline
func Fill(dst []byte) {
	x := seed
	for i := range dst {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		dst[i] = byte(x >> 56)
	}
}

// SearchPattern reconstructs the exact sequence sought in a memory dump.
func SearchPattern() []byte {
	b := make([]byte, SearchSize)
	Fill(b)
	return b
}
