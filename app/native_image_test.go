package main

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestEncodeSixel(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	source.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	source.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 255})
	source.SetNRGBA(0, 1, color.NRGBA{B: 255, A: 255})
	source.SetNRGBA(1, 1, color.NRGBA{R: 255, G: 255, B: 255, A: 255})

	encoded, err := encodeSixel(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(encoded, []byte("\x1bPq\"1;1;2;2")) {
		t.Fatalf("missing Sixel header and raster size: %q", encoded)
	}
	if !bytes.HasSuffix(encoded, []byte("\x1b\\")) {
		t.Fatalf("missing Sixel terminator: %q", encoded)
	}
	if !bytes.Contains(encoded, []byte("#")) {
		t.Fatalf("missing Sixel palette/data selectors: %q", encoded)
	}
}

func TestWriteNativeImageProtocols(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	source.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	source.SetNRGBA(1, 0, color.NRGBA{G: 255, A: 255})
	source.SetNRGBA(0, 1, color.NRGBA{B: 255, A: 255})
	source.SetNRGBA(1, 1, color.NRGBA{R: 255, G: 255, B: 255, A: 255})

	tests := []struct {
		name     string
		protocol nativeImageProtocol
		marker   []byte
	}{
		{name: "sixel", protocol: nativeImageSixel, marker: []byte("\x1bPq")},
		{name: "kitty", protocol: nativeImageKitty, marker: []byte("\x1b_G")},
		{name: "iterm", protocol: nativeImageIterm, marker: []byte("\x1b]1337;File=")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := writeNativeImage(&output, test.protocol, source, 100, 40); err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(output.Bytes(), test.marker) {
				t.Fatalf("%s output does not contain protocol marker %q: %q", test.name, test.marker, output.Bytes())
			}
		})
	}
}

func TestScaleNativeImageEnlargesSmallScreenshots(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 216, 189))
	resized := scaleNativeImage(source, 960, 720, 2)
	if resized.Bounds().Dx() != 432 || resized.Bounds().Dy() != 378 {
		t.Fatalf("native screenshot size = %v, want 432x378", resized.Bounds())
	}
	limited := scaleNativeImage(source, 300, 200, 2)
	if limited.Bounds().Dx() > 300 || limited.Bounds().Dy() > 200 {
		t.Fatalf("scaled image exceeded viewport: %v", limited.Bounds())
	}
}

func TestNativeImageSupportDetectsWindowsTerminalSession(t *testing.T) {
	clearNativeImageEnvironment(t)
	t.Setenv("WT_SESSION", "test-session")
	t.Setenv("TERM", "xterm-mintty")
	if !nativeImageSupported() {
		t.Fatal("Windows Terminal session was not detected for native image support")
	}
	if got := detectNativeImageProtocol(); got != nativeImageSixel {
		t.Fatalf("Windows Terminal backend = %s, want Sixel", got)
	}
}

func TestNativeImageProtocolDetectionAndOverride(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected nativeImageProtocol
	}{
		{name: "kitty", env: map[string]string{"TERM": "xterm-kitty"}, expected: nativeImageKitty},
		{name: "wezterm", env: map[string]string{"TERM_PROGRAM": "WezTerm"}, expected: nativeImageKitty},
		{name: "iterm", env: map[string]string{"LC_TERMINAL": "iTerm2"}, expected: nativeImageIterm},
		{name: "standalone mintty", env: map[string]string{"TERM": "mintty"}, expected: nativeImageIterm},
		{name: "wezterm outranks inherited Windows Terminal session", env: map[string]string{"WT_SESSION": "inherited-session", "TERM_PROGRAM": "WezTerm", "TERM": "xterm-256color"}, expected: nativeImageKitty},
		{name: "forced kitty", env: map[string]string{"WT_SESSION": "windows-terminal", "GEEK_LIFE_IMAGE_PROTOCOL": "kitty"}, expected: nativeImageKitty},
		{name: "forced compatibility", env: map[string]string{"WT_SESSION": "windows-terminal", "GEEK_LIFE_IMAGE_PROTOCOL": "cell"}, expected: nativeImageNone},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearNativeImageEnvironment(t)
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			if got := detectNativeImageProtocol(); got != test.expected {
				t.Fatalf("backend = %s, want %s", got, test.expected)
			}
		})
	}
}

func clearNativeImageEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"GEEK_LIFE_IMAGE_PROTOCOL", "WT_SESSION", "KITTY_WINDOW_ID", "TERM", "TERM_PROGRAM", "LC_TERMINAL"} {
		t.Setenv(key, "")
	}
}
