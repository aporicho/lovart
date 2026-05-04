package signing

import (
	"crypto/rand"
	"fmt"
)

// RandomHex generates a random hex string of length n (n must be even).
func RandomHex(n int) string {
	b := make([]byte, n/2)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
