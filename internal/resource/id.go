package resource

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewID returns a new unique resource ID of the form "<prefix>_<16 hex chars>".
// The 8 random bytes (64 bits of entropy) are sourced from crypto/rand.
func NewID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is a hard system error; panic rather than silently
		// issuing duplicate IDs.
		panic(fmt.Sprintf("resource.NewID: crypto/rand failed: %v", err))
	}
	return prefix + "_" + hex.EncodeToString(b)
}
