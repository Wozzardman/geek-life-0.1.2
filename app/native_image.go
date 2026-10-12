package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/BourgeoisBear/rasterm"
)

type nativeImageProtocol uint8

const (
	nativeImageNone nativeImageProtocol = iota
	nativeImageSixel
	nativeImageKitty
	nativeImageIterm
)

func (protocol nativeImageProtocol) String() string {
	switch protocol {
	case nativeImageSixel:
		return "Sixel"
	case nativeImageKitty:
		return "Kitty graphics"
	case nativeImageIterm:
		return "iTerm images"
	default:
		return "unavailable"
	}
}

type sixelColor struct {
	red, green, blue uint8
}

type colorAccumulator struct {
	red, green, blue uint64
	count            uint64
}

func nativeImageSupported() bool {
	return detectNativeImageProtocol() != nativeImageNone
}

func detectNativeImageProtocol() nativeImageProtocol {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GEEK_LIFE_IMAGE_PROTOCOL"))) {
	case "sixel":
		return nativeImageSixel
	case "kitty":
		return nativeImageKitty
	case "iterm", "iterm2":
		return nativeImageIterm
	case "cell", "compat", "off", "none":
		return nativeImageNone
	}

	// TERM_PROGRAM and KITTY_WINDOW_ID identify the process that is actually
	// drawing the terminal. They must outrank WT_SESSION, which can remain in
	// the environment when WezTerm is launched from Windows Terminal.
	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	lcTerminal := strings.ToLower(os.Getenv("LC_TERMINAL"))
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return nativeImageKitty
	}
	for _, marker := range []string{"xterm-kitty", "wezterm", "ghostty"} {
		if strings.Contains(termProgram, marker) {
			return nativeImageKitty
		}
	}
	for _, marker := range []string{"iterm", "mintty", "rio"} {
		if strings.Contains(termProgram+" "+lcTerminal, marker) {
			return nativeImageIterm
		}
	}

	// Windows Terminal 1.22+ supports Sixel regardless of the shell hosted in
	// the tab. Check it before weaker TERM hints, since Git Bash can advertise
	// mintty-like values even when Windows Terminal is the actual renderer.
	if os.Getenv("WT_SESSION") != "" {
		return nativeImageSixel
	}
	terminal := strings.ToLower(strings.Join([]string{
		os.Getenv("TERM"),
		termProgram,
		lcTerminal,
	}, " "))
	if strings.Contains(terminal, "xterm-kitty") {
		return nativeImageKitty
	}
	for _, marker := range []string{"mintty", "rio"} {
		if strings.Contains(terminal, marker) {
			return nativeImageIterm
		}
	}
	for _, marker := range []string{"sixel", "mlterm", "foot", "contour"} {
		if strings.Contains(terminal, marker) {
			return nativeImageSixel
		}
	}
	return nativeImageNone
}

func imageProtocolDiagnostic() string {
	protocol := detectNativeImageProtocol()
	override := strings.TrimSpace(os.Getenv("GEEK_LIFE_IMAGE_PROTOCOL"))
	if override == "" {
		override = "automatic"
	}
	return fmt.Sprintf("Image backend: %s\nBackend override: %s\nWindows Terminal session: %t\nTERM: %s\nTERM_PROGRAM: %s\nLC_TERMINAL: %s",
		protocol,
		override,
		os.Getenv("WT_SESSION") != "",
		emptyDiagnosticValue(os.Getenv("TERM")),
		emptyDiagnosticValue(os.Getenv("TERM_PROGRAM")),
		emptyDiagnosticValue(os.Getenv("LC_TERMINAL")),
	)
}

func emptyDiagnosticValue(value string) string {
	if value == "" {
		return "(unset)"
	}
	return value
}

func displayNativeImage(path string, columns, rows int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	source, _, err := image.Decode(file)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}

	if columns < 40 {
		columns = 100
	}
	if rows < 12 {
		rows = 40
	}
	// These conservative cell dimensions match common terminal fonts. The
	// viewer enlarges small screenshots 2x while keeping large images on-screen.
	maxWidth := minInt(columns*8, 1200)
	maxHeight := minInt((rows-5)*16, 800)
	resized := scaleNativeImage(source, maxWidth, maxHeight, 2)
	protocol := detectNativeImageProtocol()
	if protocol == nativeImageNone {
		return fmt.Errorf("this terminal does not advertise a native image protocol")
	}
	var encoded bytes.Buffer
	if err := writeNativeImage(&encoded, protocol, resized, columns, rows); err != nil {
		return err
	}

	imageRows := (resized.Bounds().Dy() + 15) / 16
	promptRow := minInt(rows, imageRows+4)
	fmt.Fprintf(os.Stdout, "\x1b[2J\x1b[H[Geek-life pixels: %s]  2x when space permits\r\n", protocol)
	if _, err := encoded.WriteTo(os.Stdout); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "\x1b[%d;1HPress Enter to return to Geek-life...", promptRow)
	waitForNativeImageKey()
	if protocol == nativeImageKitty {
		// Kitty placements survive a normal text-screen clear unless explicitly
		// deleted. Remove every placement created by this temporary viewer.
		fmt.Fprint(os.Stdout, "\x1b_Ga=d,d=A\x1b\\")
	}
	fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H")
	return nil
}

func writeNativeImage(out io.Writer, protocol nativeImageProtocol, source image.Image, columns, rows int) error {
	width, height := source.Bounds().Dx(), source.Bounds().Dy()
	switch protocol {
	case nativeImageSixel:
		encoded, err := encodeSixel(source)
		if err != nil {
			return err
		}
		_, err = out.Write(encoded)
		return err
	case nativeImageKitty:
		return rasterm.KittyWriteImage(out, source, rasterm.KittyImgOpts{
			DstCols: uint32(minInt(columns, (width+7)/8)),
			DstRows: uint32(minInt(maxInt(1, rows-5), (height+15)/16)),
		})
	case nativeImageIterm:
		var encodedPNG bytes.Buffer
		if err := png.Encode(&encodedPNG, source); err != nil {
			return err
		}
		return rasterm.ItermCopyFileInline(out, &encodedPNG, int64(encodedPNG.Len()))
	default:
		return fmt.Errorf("unsupported native image protocol")
	}
}

func scaleNativeImage(source image.Image, maxWidth, maxHeight, maximumScale int) image.Image {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	scale := float64(maximumScale)
	if scale > float64(maxWidth)/float64(width) {
		scale = float64(maxWidth) / float64(width)
	}
	if scale > float64(maxHeight)/float64(height) {
		scale = float64(maxHeight) / float64(height)
	}
	if scale <= 0 {
		scale = 1
	}
	targetWidth := maxInt(1, int(float64(width)*scale))
	targetHeight := maxInt(1, int(float64(height)*scale))
	destination := image.NewNRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		sourceY := bounds.Min.Y + minInt(height-1, int(float64(y)/scale))
		for x := 0; x < targetWidth; x++ {
			sourceX := bounds.Min.X + minInt(width-1, int(float64(x)/scale))
			destination.SetNRGBA(x, y, compositeOnWhite(color.NRGBAModel.Convert(source.At(sourceX, sourceY)).(color.NRGBA)))
		}
	}
	return destination
}

func compositeOnWhite(pixel color.NRGBA) color.NRGBA {
	if pixel.A == 255 {
		return pixel
	}
	alpha := uint32(pixel.A)
	return color.NRGBA{
		R: uint8((uint32(pixel.R)*alpha + 255*(255-alpha)) / 255),
		G: uint8((uint32(pixel.G)*alpha + 255*(255-alpha)) / 255),
		B: uint8((uint32(pixel.B)*alpha + 255*(255-alpha)) / 255),
		A: 255,
	}
}

// encodeSixel uses a compact 64-color adaptive palette. Sixel is supported by
// current Windows Terminal and a number of other modern terminal emulators.
func encodeSixel(source image.Image) ([]byte, error) {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("image has no pixels")
	}

	codes := make([]uint8, width*height)
	accumulators := make(map[uint8]*colorAccumulator)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixel := compositeOnWhite(color.NRGBAModel.Convert(source.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA))
			code := (pixel.R>>6)<<4 | (pixel.G>>6)<<2 | pixel.B>>6
			codes[y*width+x] = code
			accumulator := accumulators[code]
			if accumulator == nil {
				accumulator = &colorAccumulator{}
				accumulators[code] = accumulator
			}
			accumulator.red += uint64(pixel.R)
			accumulator.green += uint64(pixel.G)
			accumulator.blue += uint64(pixel.B)
			accumulator.count++
		}
	}

	orderedCodes := make([]int, 0, len(accumulators))
	for code := range accumulators {
		orderedCodes = append(orderedCodes, int(code))
	}
	sort.Ints(orderedCodes)
	palette := make([]sixelColor, len(orderedCodes))
	paletteIndex := make(map[uint8]uint8, len(orderedCodes))
	for index, rawCode := range orderedCodes {
		code := uint8(rawCode)
		accumulator := accumulators[code]
		palette[index] = sixelColor{
			red:   uint8(accumulator.red / accumulator.count),
			green: uint8(accumulator.green / accumulator.count),
			blue:  uint8(accumulator.blue / accumulator.count),
		}
		paletteIndex[code] = uint8(index)
	}
	for i, code := range codes {
		codes[i] = paletteIndex[code]
	}

	var output bytes.Buffer
	output.Grow(width * height / 2)
	output.WriteString("\x1bPq")
	fmt.Fprintf(&output, "\"1;1;%d;%d", width, height)
	for index, entry := range palette {
		fmt.Fprintf(&output, "#%d;2;%d;%d;%d", index, colorPercent(entry.red), colorPercent(entry.green), colorPercent(entry.blue))
	}

	for bandY := 0; bandY < height; bandY += 6 {
		used := make([]bool, len(palette))
		for y := bandY; y < minInt(height, bandY+6); y++ {
			for x := 0; x < width; x++ {
				used[codes[y*width+x]] = true
			}
		}
		firstPlane := true
		for paletteEntry := range palette {
			if !used[paletteEntry] {
				continue
			}
			if !firstPlane {
				output.WriteByte('$')
			}
			firstPlane = false
			fmt.Fprintf(&output, "#%d", paletteEntry)
			line := make([]byte, width)
			lastNonEmpty := -1
			for x := 0; x < width; x++ {
				mask := byte(0)
				for bit := 0; bit < 6 && bandY+bit < height; bit++ {
					if int(codes[(bandY+bit)*width+x]) == paletteEntry {
						mask |= 1 << uint(bit)
					}
				}
				line[x] = '?' + mask
				if mask != 0 {
					lastNonEmpty = x
				}
			}
			writeSixelRun(&output, line[:lastNonEmpty+1])
		}
		if bandY+6 < height {
			output.WriteByte('-')
		}
	}
	output.WriteString("\x1b\\")
	return output.Bytes(), nil
}

func writeSixelRun(output *bytes.Buffer, values []byte) {
	for start := 0; start < len(values); {
		end := start + 1
		for end < len(values) && values[end] == values[start] {
			end++
		}
		count := end - start
		if count >= 4 {
			fmt.Fprintf(output, "!%d%c", count, values[start])
		} else {
			for repeat := 0; repeat < count; repeat++ {
				output.WriteByte(values[start])
			}
		}
		start = end
	}
}

func colorPercent(value uint8) int {
	return (int(value)*100 + 127) / 255
}

func minInt(first, second int) int {
	if first < second {
		return first
	}
	return second
}
