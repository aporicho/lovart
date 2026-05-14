package project

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	defaultCanvasColumns     = 4
	defaultCanvasGap         = 100
	defaultCanvasPadding     = 100
	defaultCanvasTitleHeight = 200
	defaultCanvasFrameGap    = 240
)

func addBatchToCanvasJSON(jsonStr string, batch CanvasBatch) (*canvasMutation, error) {
	if jsonStr == "" {
		jsonStr = defaultCanvasJSON()
	}
	if !json.Valid([]byte(jsonStr)) {
		return nil, fmt.Errorf("invalid canvas JSON")
	}

	var err error
	jsonStr, err = ensureCanvasStore(jsonStr)
	if err != nil {
		return nil, err
	}

	sections := nonEmptyCanvasSections(batch.Sections)
	if len(sections) == 0 {
		return &canvasMutation{
			JSON:      jsonStr,
			PicCount:  countCImagesGJSON(jsonStr, canvasStorePath),
			CoverList: extractCoverListGJSON(jsonStr),
		}, nil
	}

	options := normalizeCanvasLayoutOptions(batch.Options)
	startX, startY := computeSectionLayoutStartGJSON(jsonStr, canvasStorePath, options)
	imageCount := countCImagesGJSON(jsonStr, canvasStorePath)
	frameIndices := indicesForNewSiblingsGJSON(jsonStr, canvasStorePath, "page:page", len(sections))

	y := startY
	for i, section := range sections {
		layout := computeSectionLayout(section, options)

		frameID, err := newShapeID()
		if err != nil {
			return nil, fmt.Errorf("frame id: %w", err)
		}
		frameJSON, err := buildFrameNodeJSON(frameID, frameIndices[i], sectionTitle(section), startX, y, layout.FrameWidth, layout.FrameHeight)
		if err != nil {
			return nil, fmt.Errorf("build frame: %w", err)
		}
		jsonStr, err = sjson.SetRaw(jsonStr, canvasStorePath+"."+frameID, frameJSON)
		if err != nil {
			return nil, fmt.Errorf("insert frame: %w", err)
		}

		childIndices := indicesForPositions(len(section.Images) + 1)
		textID, err := newShapeID()
		if err != nil {
			return nil, fmt.Errorf("text id: %w", err)
		}
		textJSON, err := buildTextNodeJSON(textID, frameID, childIndices[0], sectionText(section), options.Padding, options.Padding)
		if err != nil {
			return nil, fmt.Errorf("build text: %w", err)
		}
		jsonStr, err = sjson.SetRaw(jsonStr, canvasStorePath+"."+textID, textJSON)
		if err != nil {
			return nil, fmt.Errorf("insert text: %w", err)
		}

		for j, item := range layout.Images {
			imageID, err := newShapeID()
			if err != nil {
				return nil, fmt.Errorf("image id: %w", err)
			}
			imageCount++
			name := fmt.Sprintf(" Image %d", imageCount)
			nodeJSON, err := buildImageNodeJSON(item.Image, imageID, childIndices[j+1], name, frameID, item.X, item.Y, item.Width, item.Height)
			if err != nil {
				return nil, fmt.Errorf("build image: %w", err)
			}
			jsonStr, err = sjson.SetRaw(jsonStr, canvasStorePath+"."+imageID, nodeJSON)
			if err != nil {
				return nil, fmt.Errorf("insert image: %w", err)
			}
		}

		y += layout.FrameHeight + options.FrameGap
	}

	mutated := &canvasMutation{
		JSON:      jsonStr,
		PicCount:  countCImagesGJSON(jsonStr, canvasStorePath),
		CoverList: extractCoverListGJSON(jsonStr),
	}
	mutated, _, err = normalizeCanvasJSON(mutated.JSON)
	if err != nil {
		return nil, err
	}
	return mutated, nil
}

func ensureCanvasStore(jsonStr string) (string, error) {
	if gjson.Get(jsonStr, canvasStorePath).Exists() {
		return jsonStr, nil
	}
	updated, err := sjson.SetRaw(jsonStr, canvasStorePath, "{}")
	if err != nil {
		return "", fmt.Errorf("create store: %w", err)
	}
	return updated, nil
}

func nonEmptyCanvasSections(sections []CanvasSection) []CanvasSection {
	out := make([]CanvasSection, 0, len(sections))
	for _, section := range sections {
		if len(section.Images) > 0 {
			out = append(out, section)
		}
	}
	return out
}

func normalizeCanvasLayoutOptions(options CanvasLayoutOptions) CanvasLayoutOptions {
	if options.Columns <= 0 {
		options.Columns = defaultCanvasColumns
	}
	if options.Gap <= 0 {
		options.Gap = defaultCanvasGap
	}
	if options.Padding <= 0 {
		options.Padding = defaultCanvasPadding
	}
	if options.TitleHeight <= 0 {
		options.TitleHeight = defaultCanvasTitleHeight
	}
	if options.FrameGap <= 0 {
		options.FrameGap = defaultCanvasFrameGap
	}
	return options
}

type sectionLayout struct {
	FrameWidth  int
	FrameHeight int
	Images      []sectionImageLayout
}

type sectionImageLayout struct {
	Image  CanvasImage
	X      int
	Y      int
	Width  int
	Height int
}

func computeSectionLayout(section CanvasSection, options CanvasLayoutOptions) sectionLayout {
	columns := options.Columns
	if len(section.Images) < columns {
		columns = len(section.Images)
	}
	if columns <= 0 {
		columns = 1
	}

	sizes := make([]sectionImageLayout, 0, len(section.Images))
	tileW := 1
	tileH := 1
	for _, img := range section.Images {
		w, h := displayImageSize(img, options.ImageMaxSide)
		if w > tileW {
			tileW = w
		}
		if h > tileH {
			tileH = h
		}
		sizes = append(sizes, sectionImageLayout{Image: img, Width: w, Height: h})
	}

	imageY := options.Padding + options.TitleHeight
	for i := range sizes {
		col := i % columns
		row := i / columns
		sizes[i].X = options.Padding + col*(tileW+options.Gap)
		sizes[i].Y = imageY + row*(tileH+options.Gap)
	}

	rows := int(math.Ceil(float64(len(section.Images)) / float64(columns)))
	frameWidth := options.Padding*2 + columns*tileW + (columns-1)*options.Gap
	frameHeight := imageY + rows*tileH + (rows-1)*options.Gap + options.Padding
	return sectionLayout{FrameWidth: frameWidth, FrameHeight: frameHeight, Images: sizes}
}

func displayImageSize(img CanvasImage, maxSide int) (int, int) {
	w := img.Width
	if w <= 0 {
		w = 1024
	}
	h := img.Height
	if h <= 0 {
		h = 1024
	}
	if maxSide <= 0 {
		return w, h
	}
	longSide := w
	if h > longSide {
		longSide = h
	}
	if longSide <= maxSide {
		return w, h
	}
	scale := float64(maxSide) / float64(longSide)
	scaledW := int(math.Round(float64(w) * scale))
	scaledH := int(math.Round(float64(h) * scale))
	if scaledW < 1 {
		scaledW = 1
	}
	if scaledH < 1 {
		scaledH = 1
	}
	return scaledW, scaledH
}

func sectionTitle(section CanvasSection) string {
	if strings.TrimSpace(section.Title) != "" {
		return section.Title
	}
	if strings.TrimSpace(section.ID) != "" {
		return section.ID
	}
	return "Generated images"
}

func sectionText(section CanvasSection) string {
	title := sectionTitle(section)
	if strings.TrimSpace(section.Subtitle) == "" {
		return title
	}
	return title + " · " + section.Subtitle
}
