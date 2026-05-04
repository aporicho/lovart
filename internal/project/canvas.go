package project

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"

	"github.com/aporicho/lovart/internal/http"
)

const canvasPrefix = "SHAKKERDATA://"

// CanvasImage is one generated image to place on the project canvas.
type CanvasImage struct {
	TaskID string
	URL    string
	Width  int
	Height int
}

// AddToCanvas adds generated images as nodes on the project canvas.
// Images are placed in a grid appended to the right of existing content.
func AddToCanvas(ctx context.Context, client *http.Client, projectID, cid string, images []CanvasImage) error {
	if len(images) == 0 {
		return nil
	}

	// 1. Load current canvas state.
	canvasData, version, err := queryCanvas(ctx, client, projectID, cid)
	if err != nil {
		return fmt.Errorf("canvas: query: %w", err)
	}

	// 2. Decode the full canvas document.
	fullDoc, store, err := decodeCanvas(canvasData)
	if err != nil {
		return fmt.Errorf("canvas: decode: %w", err)
	}
	if store == nil {
		store = make(map[string]any)
		// If no document, create minimal structure.
		if fullDoc == nil {
			fullDoc = map[string]any{
				"tldrawSnapshot": map[string]any{
					"document": map[string]any{
						"store":  store,
						"schema": map[string]any{"schemaVersion": 2, "sequences": []any{}},
					},
					"session": map[string]any{},
				},
			}
		}
	}

	// 3. Compute layout.
	startX, startY := computeLayout(store)

	// 4. Add nodes.
	columns := 4
	gap := 64
	for i, img := range images {
		col := i % columns
		row := i / columns
		x := startX + col*(img.Width+gap)
		y := startY + row*(img.Height+gap)

		node := buildNode(img, x, y)
		store[node["id"].(string)] = node
	}

	// 5. Re-encode preserving the full document structure.
	newCanvas, err := encodeCanvas(fullDoc)
	if err != nil {
		return fmt.Errorf("canvas: encode: %w", err)
	}

	if err := saveCanvas(ctx, client, projectID, cid, newCanvas, version); err != nil {
		return fmt.Errorf("canvas: save: %w", err)
	}

	return nil
}

// queryCanvas loads the current canvas SHAKKERDATA and version from the API.
func queryCanvas(ctx context.Context, client *http.Client, projectID, cid string) (string, string, error) {
	path := "/api/canva/project/queryProject"

	body := map[string]any{
		"projectId": projectID,
		"cid":       cid,
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Canvas  string `json:"canvas"`
			Version string `json:"version"`
		} `json:"data"`
	}

	if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
		return "", "", fmt.Errorf("canvas: query project: %w", err)
	}
	if resp.Code != 0 {
		return "", "", fmt.Errorf("canvas: query project returned code %d", resp.Code)
	}

	return resp.Data.Canvas, resp.Data.Version, nil
}

// saveCanvas uploads the updated canvas to Lovart.
func saveCanvas(ctx context.Context, client *http.Client, projectID, cid, canvasData, _ string) error {
	path := "/api/canva/project/saveProject"

	body := map[string]any{
		"projectType": 3,
		"cid":         cid,
		"canvas":      canvasData,
		"projectId":   projectID,
	}

	var resp struct {
		Code int `json:"code"`
	}

	if err := client.PostJSON(ctx, http.WWWBase, path, body, &resp); err != nil {
		return fmt.Errorf("canvas: save project: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("canvas: save project returned code %d", resp.Code)
	}
	return nil
}

// decodeCanvas parses a SHAKKERDATA string and returns the full document and its store.
func decodeCanvas(data string) (map[string]any, map[string]any, error) {
	if data == "" || len(data) < len(canvasPrefix) {
		return nil, make(map[string]any), nil
	}

	raw := data[len(canvasPrefix):]
	padded := raw + "===="[:(-len(raw)%4)]

	decoded, err := base64.StdEncoding.DecodeString(padded)
	if err != nil {
		return nil, nil, fmt.Errorf("canvas: base64 decode: %w", err)
	}

	reader, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return nil, nil, fmt.Errorf("canvas: gzip reader: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("canvas: gzip read: %w", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(decompressed, &doc); err != nil {
		return nil, nil, fmt.Errorf("canvas: json parse: %w", err)
	}

	// Navigate to the store.
	snapshot, _ := doc["tldrawSnapshot"].(map[string]any)
	if snapshot == nil {
		return doc, make(map[string]any), nil
	}

	document, _ := snapshot["document"].(map[string]any)
	if document == nil {
		return doc, make(map[string]any), nil
	}

	store, _ := document["store"].(map[string]any)
	if store == nil {
		store = make(map[string]any)
		document["store"] = store
	}

	return doc, store, nil
}

// encodeCanvas serializes the full canvas document back to a SHAKKERDATA string.
func encodeCanvas(doc map[string]any) (string, error) {
	jsonBytes, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("canvas: json marshal: %w", err)
	}

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(jsonBytes); err != nil {
		return "", fmt.Errorf("canvas: gzip write: %w", err)
	}
	writer.Close()

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return canvasPrefix + encoded, nil
}

// computeLayout determines where to place new nodes on the canvas.
func computeLayout(store map[string]any) (int, int) {
	maxRight := 0
	for _, v := range store {
		node, ok := v.(map[string]any)
		if !ok {
			continue
		}
		x, _ := getFloat(node, "x")
		w, _ := getFloat(node, "props", "w")
		right := int(x) + int(w)
		if right > maxRight {
			maxRight = right
		}
	}

	if maxRight == 0 {
		return 100, 100
	}
	return maxRight + 64, 0
}

// buildNode constructs a tldraw c-image shape node.
func buildNode(img CanvasImage, x, y int) map[string]any {
	id := "shape:" + randomString(22)
	index := randomString(7)

	return map[string]any{
		"x":         float64(x),
		"y":         float64(y),
		"rotation":  0,
		"isLocked":  false,
		"opacity":   1,
		"meta":      map[string]any{"source": "ai"},
		"id":        id,
		"type":      "c-image",
		"props": map[string]any{
			"w":               img.Width,
			"h":               img.Height,
			"url":             img.URL,
			"originalUrl":     img.URL,
			"radius":          0,
			"name":            " Image",
			"genType":         1,
			"generatorTaskId": img.TaskID,
		},
		"parentId": "page:page",
		"index":    index,
		"typeName": "shape",
	}
}

// getFloat reads a float64 from a nested path of map[string]any.
func getFloat(m map[string]any, path ...string) (float64, bool) {
	current := any(m)
	for _, key := range path {
		dict, ok := current.(map[string]any)
		if !ok {
			return 0, false
		}
		current = dict[key]
	}
	switch v := current.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}

// randomString generates a random alphanumeric string of length n.
func randomString(n int) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	result := make([]byte, n)
	for i := range result {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		result[i] = chars[idx.Int64()]
	}
	return string(result)
}
