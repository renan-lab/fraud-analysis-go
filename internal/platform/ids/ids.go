// Package ids generates random identifiers for events and requests.
package ids

import (
	"crypto/rand"
	"fmt"
)

// NewV4 returns a random lowercase UUID v4 read from crypto/rand.
//
//	eventID := ids.NewV4() // "7c9e6679-7425-40de-944b-e07fc1f90ae7"
func NewV4() string {
	var b [16]byte
	// crypto/rand.Read never returns an error since Go 1.24; it crashes the program instead.
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
