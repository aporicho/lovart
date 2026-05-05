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

// CanvasSection is one logical canvas group, usually one batch job.
type CanvasSection struct {
	ID       string
	Title    string
	Subtitle string
	Images   []CanvasImage
}

// CanvasBatch is a single canvas write containing multiple logical sections.
type CanvasBatch struct {
	Sections []CanvasSection
	Options  CanvasLayoutOptions
}

// CanvasLayoutOptions controls how generated sections are placed on the canvas.
type CanvasLayoutOptions struct {
	Columns      int
	Gap          int
	Padding      int
	TitleHeight  int
	FrameGap     int
	ImageMaxSide int
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
