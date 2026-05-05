package project

const canvasPrefix = "SHAKKERDATA://"
const canvasStorePath = "tldrawSnapshot.document.store"

// CanvasImage is one generated image to place on the project canvas.
type CanvasImage struct {
	TaskID string
	URL    string
	Width  int
	Height int
}

// canvasState holds all fields required by saveProject.
type canvasState struct {
	Canvas    string
	Version   string
	Name      string
	PicCount  int
	CoverList []string
}

type canvasMutation struct {
	JSON      string
	PicCount  int
	CoverList []string
}
