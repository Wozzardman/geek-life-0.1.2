package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ImagePreview renders an image using upper-half block characters. Each
// terminal cell carries two vertical image samples through foreground and
// background true color, so it works without a terminal-specific image API.
type ImagePreview struct {
	*tview.Box
	paths   []string
	current int
	image   image.Image
	loadErr error
	zoom    int
	panX    float64
	panY    float64
	detail  bool
	modeSet bool
}

func NewImagePreview() *ImagePreview {
	preview := &ImagePreview{Box: tview.NewBox(), zoom: 1, panX: 0.5, panY: 0.5}
	preview.SetBorder(true).SetBorderColor(tcell.ColorLightSlateGray)
	return preview
}

func (preview *ImagePreview) SetImages(paths []string) {
	preview.paths = append(preview.paths[:0], paths...)
	if len(preview.paths) == 0 {
		preview.current = 0
		preview.image = nil
		preview.loadErr = nil
		preview.detail = false
		preview.modeSet = false
		preview.SetTitle(" Image preview ")
		return
	}

	preview.current = len(preview.paths) - 1
	preview.modeSet = false
	preview.Fit()
	preview.loadCurrent()
}

func (preview *ImagePreview) HasImages() bool {
	return len(preview.paths) != 0
}

func (preview *ImagePreview) Move(delta int) {
	if len(preview.paths) < 2 {
		return
	}
	preview.current = (preview.current + delta + len(preview.paths)) % len(preview.paths)
	preview.Fit()
	preview.loadCurrent()
}

func (preview *ImagePreview) Zoom(delta int) {
	preview.SetZoom(preview.zoom + delta)
}

func (preview *ImagePreview) SetZoom(zoom int) {
	preview.zoom = zoom
	if preview.zoom < 1 {
		preview.zoom = 1
	}
	if preview.zoom > 16 {
		preview.zoom = 16
	}
	preview.updateTitle()
}

func (preview *ImagePreview) Pan(dx, dy float64) {
	preview.panX = math.Max(0, math.Min(1, preview.panX+dx))
	preview.panY = math.Max(0, math.Min(1, preview.panY+dy))
}

// ToggleDetailMode switches between true-color half blocks and a higher
// resolution contiguous-block renderer designed for screenshots and text.
func (preview *ImagePreview) ToggleDetailMode() {
	preview.detail = !preview.detail
	preview.modeSet = true
	preview.updateTitle()
}

func (preview *ImagePreview) Fit() {
	preview.zoom, preview.panX, preview.panY = 1, 0.5, 0.5
	preview.updateTitle()
}

func (preview *ImagePreview) CurrentPath() string {
	if len(preview.paths) == 0 {
		return ""
	}
	return preview.paths[preview.current]
}

func (preview *ImagePreview) loadCurrent() {
	path := preview.paths[preview.current]
	preview.updateTitle()

	file, err := os.Open(path)
	if err != nil {
		preview.image, preview.loadErr = nil, err
		return
	}
	defer file.Close()

	preview.image, _, preview.loadErr = image.Decode(file)
	// The portable viewer defaults to widely supported half blocks. Sextant
	// glyphs are available manually with b, since font coverage varies between
	// terminal profiles and missing glyphs render as replacement characters.
	if preview.loadErr == nil && !preview.modeSet {
		preview.detail = false
	}
	preview.updateTitle()
}

func (preview *ImagePreview) updateTitle() {
	if len(preview.paths) == 0 {
		preview.SetTitle(" Image preview ")
		return
	}
	mode := "color"
	if preview.detail {
		mode = "detail"
	}
	preview.SetTitle(fmt.Sprintf(" Image %d/%d: %s — %dx %s ", preview.current+1, len(preview.paths), filepath.Base(preview.paths[preview.current]), preview.zoom, mode))
}

// InputHandler allows ImagePreview to be focused in the full-terminal viewer.
func (preview *ImagePreview) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return preview.WrapInputHandler(func(_ *tcell.EventKey, _ func(p tview.Primitive)) {})
}

func (preview *ImagePreview) Draw(screen tcell.Screen) {
	preview.Box.DrawForSubclass(screen, preview)
	x, y, width, height := preview.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}
	if preview.loadErr != nil {
		tview.Print(screen, "Image unavailable: "+preview.loadErr.Error(), x, y, width, tview.AlignCenter, tcell.ColorIndianRed)
		return
	}
	if preview.image == nil {
		tview.Print(screen, "No pasted image", x, y, width, tview.AlignCenter, tcell.ColorDimGray)
		return
	}
	if preview.detail {
		preview.drawSextant(screen, x, y, width, height)
		return
	}

	bounds := preview.image.Bounds()
	sourceWidth, sourceHeight := bounds.Dx(), bounds.Dy()
	if sourceWidth == 0 || sourceHeight == 0 {
		return
	}

	maxPixelHeight := height * 2
	fitScaleWidth := float64(width) / float64(sourceWidth)
	fitScaleHeight := float64(maxPixelHeight) / float64(sourceHeight)
	fitScale := fitScaleWidth
	if fitScaleHeight < fitScale {
		fitScale = fitScaleHeight
	}
	scale := fitScale * float64(preview.zoom)
	drawWidth := int(float64(sourceWidth) * scale)
	drawPixelHeight := int(float64(sourceHeight) * scale)
	if drawWidth < 1 {
		drawWidth = 1
	}
	if drawPixelHeight < 1 {
		drawPixelHeight = 1
	}

	destinationX, destinationY := 0, 0
	sourceX, sourceY := 0.0, 0.0
	visibleWidth, visibleHeight := float64(sourceWidth), float64(sourceHeight)
	if drawWidth <= width {
		destinationX = (width - drawWidth) / 2
	} else {
		drawWidth = width
		visibleWidth = float64(width) / scale
		sourceX = preview.panX * (float64(sourceWidth) - visibleWidth)
	}
	if drawPixelHeight <= maxPixelHeight {
		destinationY = (maxPixelHeight - drawPixelHeight) / 2
	} else {
		drawPixelHeight = maxPixelHeight
		visibleHeight = float64(maxPixelHeight) / scale
		sourceY = preview.panY * (float64(sourceHeight) - visibleHeight)
	}

	for cellY := 0; cellY < height; cellY++ {
		for cellX := 0; cellX < width; cellX++ {
			top, topOK := preview.sampleDestination(bounds, cellX, cellY*2, destinationX, destinationY, drawWidth, drawPixelHeight, sourceX, sourceY, scale)
			bottom, bottomOK := preview.sampleDestination(bounds, cellX, cellY*2+1, destinationX, destinationY, drawWidth, drawPixelHeight, sourceX, sourceY, scale)
			if !topOK && !bottomOK {
				continue
			}
			if !topOK {
				top = tcell.ColorBlack
			}
			if !bottomOK {
				bottom = tcell.ColorBlack
			}
			style := tcell.StyleDefault.Foreground(top).Background(bottom)
			screen.SetContent(x+cellX, y+cellY, '▀', nil, style)
		}
	}
}

// drawSextant renders six contiguous sub-cells per terminal character. Unlike
// Braille, the sub-cells touch, so the strokes in small UI fonts remain joined.
func (preview *ImagePreview) drawSextant(screen tcell.Screen, x, y, width, height int) {
	bounds := preview.image.Bounds()
	sourceWidth, sourceHeight := bounds.Dx(), bounds.Dy()
	if sourceWidth == 0 || sourceHeight == 0 {
		return
	}

	maxDotWidth, maxDotHeight := width*2, height*3
	// A terminal cell is usually about twice as tall as it is wide. Sextants
	// divide it 2x3, so their horizontal scale needs a 4:3 correction to keep
	// the source image's physical aspect ratio intact.
	const horizontalCorrection = 4.0 / 3.0
	fitScale := math.Min(
		float64(maxDotWidth)/(float64(sourceWidth)*horizontalCorrection),
		float64(maxDotHeight)/float64(sourceHeight),
	)
	scaleY := fitScale * float64(preview.zoom)
	scaleX := scaleY * horizontalCorrection
	drawDotWidth := maxInt(1, int(float64(sourceWidth)*scaleX))
	drawDotHeight := maxInt(1, int(float64(sourceHeight)*scaleY))

	destinationX, destinationY := 0, 0
	sourceX, sourceY := 0.0, 0.0
	if drawDotWidth <= maxDotWidth {
		destinationX = (maxDotWidth - drawDotWidth) / 2
	} else {
		drawDotWidth = maxDotWidth
		visibleWidth := float64(maxDotWidth) / scaleX
		sourceX = preview.panX * (float64(sourceWidth) - visibleWidth)
	}
	if drawDotHeight <= maxDotHeight {
		destinationY = (maxDotHeight - drawDotHeight) / 2
	} else {
		drawDotHeight = maxDotHeight
		visibleHeight := float64(maxDotHeight) / scaleY
		sourceY = preview.panY * (float64(sourceHeight) - visibleHeight)
	}

	backgroundLuminance := preview.imageLuminance(bounds)
	lightBackground := backgroundLuminance >= 128
	threshold := backgroundLuminance - 35
	style := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorWhite)
	if !lightBackground {
		threshold = backgroundLuminance + 35
		style = tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
	}

	dotMasks := [3][2]rune{{0x01, 0x02}, {0x04, 0x08}, {0x10, 0x20}}
	for cellY := 0; cellY < height; cellY++ {
		for cellX := 0; cellX < width; cellX++ {
			var dots rune
			inside := false
			for dotY := 0; dotY < 3; dotY++ {
				for dotX := 0; dotX < 2; dotX++ {
					destinationDotX := cellX*2 + dotX
					destinationDotY := cellY*3 + dotY
					if destinationDotX < destinationX || destinationDotX >= destinationX+drawDotWidth || destinationDotY < destinationY || destinationDotY >= destinationY+drawDotHeight {
						continue
					}
					inside = true
					sx := sourceX + float64(destinationDotX-destinationX)/scaleX
					sy := sourceY + float64(destinationDotY-destinationY)/scaleY
					luminance := preview.sampleLuminance(bounds, sx, sy, sx+1/scaleX, sy+1/scaleY, lightBackground)
					if lightBackground && luminance < threshold || !lightBackground && luminance > threshold {
						dots |= dotMasks[dotY][dotX]
					}
				}
			}
			if !inside {
				continue
			}
			rendered := sextantRune(dots)
			screen.SetContent(x+cellX, y+cellY, rendered, nil, style)
		}
	}
}

// sextantRune maps a six-bit 2x3 bitmap to Symbols for Legacy Computing.
// Empty, full, left-half, and right-half patterns reuse older block glyphs.
func sextantRune(mask rune) rune {
	switch mask {
	case 0:
		return ' '
	case 0x15:
		return '▌'
	case 0x2a:
		return '▐'
	case 0x3f:
		return '█'
	}
	codepoint := rune(0x1fb00) + mask - 1
	if mask > 0x15 {
		codepoint--
	}
	if mask > 0x2a {
		codepoint--
	}
	return codepoint
}

func (preview *ImagePreview) imageLuminance(bounds image.Rectangle) float64 {
	const samples = 24
	var total float64
	for sampleY := 0; sampleY < samples; sampleY++ {
		for sampleX := 0; sampleX < samples; sampleX++ {
			x := bounds.Min.X + sampleX*bounds.Dx()/samples
			y := bounds.Min.Y + sampleY*bounds.Dy()/samples
			pixel := color.NRGBAModel.Convert(preview.image.At(x, y)).(color.NRGBA)
			total += rgbaLuminance(pixel, true)
		}
	}
	return total / float64(samples*samples)
}

func (preview *ImagePreview) sampleLuminance(bounds image.Rectangle, x0, y0, x1, y1 float64, lightBackground bool) float64 {
	points := [][2]float64{
		{x0 + (x1-x0)*0.25, y0 + (y1-y0)*0.25},
		{x0 + (x1-x0)*0.75, y0 + (y1-y0)*0.25},
		{x0 + (x1-x0)*0.25, y0 + (y1-y0)*0.75},
		{x0 + (x1-x0)*0.75, y0 + (y1-y0)*0.75},
	}
	var luminance float64
	for _, point := range points {
		x := int(math.Max(float64(bounds.Min.X), math.Min(float64(bounds.Max.X-1), float64(bounds.Min.X)+point[0])))
		y := int(math.Max(float64(bounds.Min.Y), math.Min(float64(bounds.Max.Y-1), float64(bounds.Min.Y)+point[1])))
		pixel := color.NRGBAModel.Convert(preview.image.At(x, y)).(color.NRGBA)
		luminance += rgbaLuminance(pixel, lightBackground)
	}
	return luminance / float64(len(points))
}

func rgbaLuminance(pixel color.NRGBA, lightBackground bool) float64 {
	background := float64(0)
	if lightBackground {
		background = 255
	}
	alpha := float64(pixel.A) / 255
	red := float64(pixel.R)*alpha + background*(1-alpha)
	green := float64(pixel.G)*alpha + background*(1-alpha)
	blue := float64(pixel.B)*alpha + background*(1-alpha)
	return 0.2126*red + 0.7152*green + 0.0722*blue
}

func maxInt(first, second int) int {
	if first > second {
		return first
	}
	return second
}

func (preview *ImagePreview) sampleDestination(bounds image.Rectangle, x, y, destinationX, destinationY, width, height int, sourceX, sourceY, scale float64) (tcell.Color, bool) {
	if x < destinationX || x >= destinationX+width || y < destinationY || y >= destinationY+height {
		return tcell.ColorBlack, false
	}
	sx0 := sourceX + float64(x-destinationX)/scale
	sy0 := sourceY + float64(y-destinationY)/scale
	sx1, sy1 := sx0+1/scale, sy0+1/scale
	return preview.sampleArea(bounds, sx0, sy0, sx1, sy1), true
}

// sampleArea uses a small multi-sample filter instead of nearest-neighbor
// sampling. This keeps text, diagrams, and thin lines more stable when fitted.
func (preview *ImagePreview) sampleArea(bounds image.Rectangle, x0, y0, x1, y1 float64) tcell.Color {
	points := [][2]float64{
		{x0 + (x1-x0)*0.25, y0 + (y1-y0)*0.25},
		{x0 + (x1-x0)*0.75, y0 + (y1-y0)*0.25},
		{x0 + (x1-x0)*0.25, y0 + (y1-y0)*0.75},
		{x0 + (x1-x0)*0.75, y0 + (y1-y0)*0.75},
	}
	var red, green, blue uint32
	for _, point := range points {
		x := int(math.Max(float64(bounds.Min.X), math.Min(float64(bounds.Max.X-1), float64(bounds.Min.X)+point[0])))
		y := int(math.Max(float64(bounds.Min.Y), math.Min(float64(bounds.Max.Y-1), float64(bounds.Min.Y)+point[1])))
		pixel := color.NRGBAModel.Convert(preview.image.At(x, y)).(color.NRGBA)
		alpha := uint32(pixel.A)
		red += uint32(pixel.R) * alpha / 255
		green += uint32(pixel.G) * alpha / 255
		blue += uint32(pixel.B) * alpha / 255
	}
	return tcell.NewRGBColor(int32(red/4), int32(green/4), int32(blue/4))
}
