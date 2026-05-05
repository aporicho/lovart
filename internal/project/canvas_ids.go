package project

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"strings"
)

const (
	canvasShapeIDPrefix = "shape:"
	canvasShapeIDLength = 21
)

// randomString generates a random alphanumeric string of length n.
func randomString(n int) (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	result := make([]byte, n)
	for i := range result {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		result[i] = chars[idx.Int64()]
	}
	return string(result), nil
}

func newShapeID() (string, error) {
	idPart, err := randomString(canvasShapeIDLength)
	if err != nil {
		return "", err
	}
	return canvasShapeIDPrefix + idPart, nil
}

func canonicalCanvasShapeID(id string) bool {
	return strings.HasPrefix(id, canvasShapeIDPrefix) && len(strings.TrimPrefix(id, canvasShapeIDPrefix)) == canvasShapeIDLength
}

// newSessionID generates a UUID-like session identifier.
func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
