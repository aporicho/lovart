package project

import (
	"encoding/json"
	"testing"
)

func TestExtractCanvasArtifactsOrdersFiltersAndIncludesRaw(t *testing.T) {
	canvas := `{
  "tldrawSnapshot": {
    "document": {
      "store": {
        "shape:frame-b": {
          "id": "shape:frame-b",
          "type": "frame",
          "parentId": "page:page",
          "index": "b",
          "x": 500,
          "y": 0,
          "props": {"name": "Later"}
        },
        "shape:image-b": {
          "id": "shape:image-b",
          "type": "c-image",
          "parentId": "shape:frame-b",
          "index": "a1",
          "x": 100,
          "y": 100,
          "props": {
            "w": 640,
            "h": 480,
            "url": "https://cdn.example/b.png",
            "originalUrl": "https://cdn.example/b-original.png",
            "name": "Image B",
            "generatorTaskId": "task-b"
          }
        },
        "shape:frame-a": {
          "id": "shape:frame-a",
          "type": "frame",
          "parentId": "page:page",
          "index": "a",
          "x": 0,
          "y": 0,
          "props": {"name": "First"}
        },
        "shape:image-a": {
          "id": "shape:image-a",
          "type": "c-image",
          "parentId": "shape:frame-a",
          "index": "a1",
          "x": 20,
          "y": 200,
          "props": {
            "w": 1024,
            "h": 768,
            "url": "https://cdn.example/a.png",
            "name": "Image A",
            "generatorTaskId": "task-a"
          }
        },
        "shape:text-a": {
          "id": "shape:text-a",
          "type": "text",
          "parentId": "shape:frame-a",
          "index": "a0",
          "x": 0,
          "y": 0,
          "props": {"name": "Title"}
        }
      }
    }
  }
}`

	result, err := extractCanvasArtifacts(canvas, CanvasArtifactsOptions{IncludeRaw: true})
	if err != nil {
		t.Fatalf("extractCanvasArtifacts: %v", err)
	}
	if result.Total != 2 || result.Count != 2 {
		t.Fatalf("counts = total %d count %d", result.Total, result.Count)
	}
	if result.ShapeTypeCounts["c-image"] != 2 || result.ShapeTypeCounts["frame"] != 2 || result.ShapeTypeCounts["text"] != 1 {
		t.Fatalf("shape counts = %#v", result.ShapeTypeCounts)
	}
	first := result.Artifacts[0]
	if first.ArtifactID != "shape:image-a" || first.Index != 1 || first.FrameID != "shape:frame-a" {
		t.Fatalf("first artifact = %#v", first)
	}
	if first.URL != "https://cdn.example/a.png" || first.TaskID != "task-a" || first.DisplayWidth != 1024 || first.DisplayHeight != 768 {
		t.Fatalf("first artifact details = %#v", first)
	}
	if !json.Valid(first.Raw) {
		t.Fatalf("raw artifact is not valid JSON: %s", first.Raw)
	}

	filtered, err := extractCanvasArtifacts(canvas, CanvasArtifactsOptions{TaskID: "task-b", Offset: 0, Limit: 1})
	if err != nil {
		t.Fatalf("filtered extractCanvasArtifacts: %v", err)
	}
	if filtered.Total != 1 || filtered.Count != 1 || filtered.Artifacts[0].ArtifactID != "shape:image-b" {
		t.Fatalf("filtered artifacts = %#v", filtered)
	}
	if len(filtered.Artifacts[0].Raw) != 0 {
		t.Fatalf("raw should be omitted by default: %#v", filtered.Artifacts[0])
	}
}

func TestExtractCanvasArtifactsPaginatesAfterStableIndexing(t *testing.T) {
	canvas := `{
  "tldrawSnapshot": {
    "document": {
      "store": {
        "shape:one": {"id":"shape:one","type":"c-image","parentId":"page:page","index":"a","x":0,"y":0,"props":{"url":"https://cdn.example/1.png","w":10,"h":10}},
        "shape:two": {"id":"shape:two","type":"c-image","parentId":"page:page","index":"b","x":0,"y":100,"props":{"url":"https://cdn.example/2.png","w":10,"h":10}}
      }
    }
  }
}`

	result, err := extractCanvasArtifacts(canvas, CanvasArtifactsOptions{Offset: 1, Limit: 1})
	if err != nil {
		t.Fatalf("extractCanvasArtifacts: %v", err)
	}
	if result.Count != 1 || result.Total != 2 {
		t.Fatalf("counts = %#v", result)
	}
	if got := result.Artifacts[0]; got.ArtifactID != "shape:two" || got.Index != 2 {
		t.Fatalf("paginated artifact = %#v", got)
	}
}
