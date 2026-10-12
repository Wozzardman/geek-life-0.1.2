//go:build windows
// +build windows

package main

import (
	"encoding/binary"
	"image/color"
	"testing"
)

func TestDecodeDIB24BitBottomUp(t *testing.T) {
	// A 2x2, bottom-up, 24-bit DIB. Each two-pixel row is padded to 8 bytes.
	data := make([]byte, 40+16)
	binary.LittleEndian.PutUint32(data[0:4], 40)
	binary.LittleEndian.PutUint32(data[4:8], 2)
	binary.LittleEndian.PutUint32(data[8:12], 2)
	binary.LittleEndian.PutUint16(data[12:14], 1)
	binary.LittleEndian.PutUint16(data[14:16], 24)

	// Stored bottom row: blue, white.
	copy(data[40:46], []byte{255, 0, 0, 255, 255, 255})
	// Stored top row: red, green.
	copy(data[48:54], []byte{0, 0, 255, 0, 255, 0})

	decoded, err := decodeDIB(data)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]color.NRGBA{
		{{R: 255, A: 255}, {G: 255, A: 255}},
		{{B: 255, A: 255}, {R: 255, G: 255, B: 255, A: 255}},
	}
	for y := range want {
		for x := range want[y] {
			got := color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA)
			if got != want[y][x] {
				t.Fatalf("pixel (%d,%d): got %#v, want %#v", x, y, got, want[y][x])
			}
		}
	}
}
