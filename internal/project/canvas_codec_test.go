package project

import "testing"

func TestCanvasEncodeDecodeRoundTrip(t *testing.T) {
	fixture := syntheticCanvasJSON()
	encoded, err := encodeCanvasJSON(fixture)
	if err != nil {
		t.Fatalf("encodeCanvasJSON returned error: %v", err)
	}
	decoded, err := decodeCanvasJSON(encoded)
	if err != nil {
		t.Fatalf("decodeCanvasJSON returned error: %v", err)
	}
	if decoded != fixture {
		t.Fatalf("decoded canvas does not match original")
	}
}

func TestDecodeCanvasJSONRejectsBadPrefix(t *testing.T) {
	if _, err := decodeCanvasJSON("not-shakker-data"); err == nil {
		t.Fatalf("decodeCanvasJSON accepted data without SHAKKERDATA prefix")
	}
}
