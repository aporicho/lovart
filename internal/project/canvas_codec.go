package project

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// decodeCanvasJSON extracts the raw JSON string from a SHAKKERDATA canvas.
func decodeCanvasJSON(data string) (string, error) {
	if data == "" {
		return "", nil
	}
	if !strings.HasPrefix(data, canvasPrefix) {
		return "", fmt.Errorf("missing %s prefix", canvasPrefix)
	}

	raw := data[len(canvasPrefix):]
	padded := raw + "===="[:(4-len(raw)%4)%4]

	decoded, err := base64.StdEncoding.DecodeString(padded)
	if err != nil {
		return "", fmt.Errorf("canvas: base64 decode: %w", err)
	}

	reader, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return "", fmt.Errorf("canvas: gzip reader: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("canvas: gzip read: %w", err)
	}

	return string(decompressed), nil
}

// encodeCanvasJSON compresses a JSON string back to SHAKKERDATA.
func encodeCanvasJSON(jsonStr string) (string, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write([]byte(jsonStr)); err != nil {
		return "", fmt.Errorf("canvas: gzip write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("canvas: gzip close: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return canvasPrefix + encoded, nil
}
