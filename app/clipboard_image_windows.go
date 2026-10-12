//go:build windows
// +build windows

package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math/bits"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	clipboardFormatDIB   = 8
	clipboardFormatDIBV5 = 17
	bitmapRGB            = 0
	bitmapBitfields      = 3
	bitmapAlphaBitfields = 6
	maxClipboardImage    = 512 * 1024 * 1024
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	ntdll                 = windows.NewLazySystemDLL("ntdll.dll")
	procOpenClipboard     = user32.NewProc("OpenClipboard")
	procCloseClipboard    = user32.NewProc("CloseClipboard")
	procGetClipboardData  = user32.NewProc("GetClipboardData")
	procIsClipboardFormat = user32.NewProc("IsClipboardFormatAvailable")
	procGlobalLock        = kernel32.NewProc("GlobalLock")
	procGlobalUnlock      = kernel32.NewProc("GlobalUnlock")
	procGlobalSize        = kernel32.NewProc("GlobalSize")
	procRtlMoveMemory     = ntdll.NewProc("RtlMoveMemory")
)

func readClipboardImage() (image.Image, error) {
	format := uintptr(clipboardFormatDIBV5)
	if available, _, _ := procIsClipboardFormat.Call(format); available == 0 {
		format = clipboardFormatDIB
		if available, _, _ := procIsClipboardFormat.Call(format); available == 0 {
			return nil, errNoClipboardImage
		}
	}

	data, err := readClipboardBytes(format)
	if err != nil {
		return nil, err
	}
	return decodeDIB(data)
}

func readClipboardBytes(format uintptr) ([]byte, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var lastErr error
	for attempt := 0; attempt < 6; attempt++ {
		if opened, _, callErr := procOpenClipboard.Call(0); opened != 0 {
			lastErr = nil
			break
		} else {
			lastErr = windowsCallError("open clipboard", callErr)
			time.Sleep(10 * time.Millisecond)
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	defer procCloseClipboard.Call()

	handle, _, callErr := procGetClipboardData.Call(format)
	if handle == 0 {
		return nil, windowsCallError("read clipboard image", callErr)
	}

	size, _, callErr := procGlobalSize.Call(handle)
	if size == 0 || size > maxClipboardImage {
		if size > maxClipboardImage {
			return nil, fmt.Errorf("clipboard image is too large (%d bytes)", size)
		}
		return nil, windowsCallError("measure clipboard image", callErr)
	}

	pointer, _, callErr := procGlobalLock.Call(handle)
	if pointer == 0 {
		return nil, windowsCallError("lock clipboard image", callErr)
	}
	defer procGlobalUnlock.Call(handle)

	data := make([]byte, int(size))
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&data[0])), pointer, size)
	return data, nil
}

func windowsCallError(action string, err error) error {
	if errno, ok := err.(syscall.Errno); ok && errno == 0 {
		return fmt.Errorf("could not %s", action)
	}
	return fmt.Errorf("could not %s: %w", action, err)
}

// decodeDIB decodes the 24-bit and 32-bit device-independent bitmap formats
// normally placed on the Windows clipboard by browsers and screenshot tools.
func decodeDIB(data []byte) (image.Image, error) {
	if len(data) < 40 {
		return nil, fmt.Errorf("clipboard image has an invalid DIB header")
	}

	headerSize := int(binary.LittleEndian.Uint32(data[0:4]))
	widthValue := int32(binary.LittleEndian.Uint32(data[4:8]))
	heightValue := int32(binary.LittleEndian.Uint32(data[8:12]))
	planes := binary.LittleEndian.Uint16(data[12:14])
	bitCount := int(binary.LittleEndian.Uint16(data[14:16]))
	compression := binary.LittleEndian.Uint32(data[16:20])

	if headerSize < 40 || headerSize > len(data) || widthValue == 0 || heightValue == 0 || planes != 1 {
		return nil, fmt.Errorf("clipboard image has unsupported DIB dimensions")
	}
	if bitCount != 24 && bitCount != 32 {
		return nil, fmt.Errorf("clipboard image uses unsupported %d-bit pixels", bitCount)
	}
	if compression != bitmapRGB && compression != bitmapBitfields && compression != bitmapAlphaBitfields {
		return nil, fmt.Errorf("clipboard image uses unsupported compression type %d", compression)
	}
	if bitCount == 24 && compression != bitmapRGB {
		return nil, fmt.Errorf("clipboard image uses unsupported 24-bit compression")
	}

	width := int64(widthValue)
	if width < 0 {
		width = -width
	}
	height := int64(heightValue)
	if height < 0 {
		height = -height
	}
	if width > 100000 || height > 100000 {
		return nil, fmt.Errorf("clipboard image dimensions are too large")
	}

	rowStride := ((width*int64(bitCount) + 31) / 32) * 4
	pixelBytes := rowStride * height
	if pixelBytes <= 0 || pixelBytes > int64(len(data)) {
		return nil, fmt.Errorf("clipboard image pixel data is truncated")
	}

	pixelOffset := headerSize
	if headerSize == 40 && (compression == bitmapBitfields || compression == bitmapAlphaBitfields) {
		maskBytes := 12
		if compression == bitmapAlphaBitfields {
			maskBytes = 16
		}
		pixelOffset += maskBytes
	}
	if int64(pixelOffset)+pixelBytes > int64(len(data)) {
		return nil, fmt.Errorf("clipboard image pixel data is truncated")
	}

	redMask, greenMask, blueMask, alphaMask := uint32(0x00ff0000), uint32(0x0000ff00), uint32(0x000000ff), uint32(0)
	if compression == bitmapBitfields || compression == bitmapAlphaBitfields {
		maskOffset := 40
		if maskOffset+12 > len(data) {
			return nil, fmt.Errorf("clipboard image color masks are truncated")
		}
		redMask = binary.LittleEndian.Uint32(data[maskOffset : maskOffset+4])
		greenMask = binary.LittleEndian.Uint32(data[maskOffset+4 : maskOffset+8])
		blueMask = binary.LittleEndian.Uint32(data[maskOffset+8 : maskOffset+12])
		if headerSize >= 56 || compression == bitmapAlphaBitfields {
			if maskOffset+16 <= len(data) {
				alphaMask = binary.LittleEndian.Uint32(data[maskOffset+12 : maskOffset+16])
			}
		}
	}

	destination := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	for y := 0; y < int(height); y++ {
		sourceY := y
		if heightValue > 0 {
			sourceY = int(height) - 1 - y
		}
		rowStart := pixelOffset + sourceY*int(rowStride)
		for x := 0; x < int(width); x++ {
			pixelStart := rowStart + x*(bitCount/8)
			var pixel color.NRGBA
			if bitCount == 24 {
				pixel = color.NRGBA{R: data[pixelStart+2], G: data[pixelStart+1], B: data[pixelStart], A: 255}
			} else {
				value := binary.LittleEndian.Uint32(data[pixelStart : pixelStart+4])
				pixel = color.NRGBA{
					R: maskedColor(value, redMask),
					G: maskedColor(value, greenMask),
					B: maskedColor(value, blueMask),
					A: 255,
				}
				if alphaMask != 0 {
					pixel.A = maskedColor(value, alphaMask)
				}
			}
			destination.SetNRGBA(x, y, pixel)
		}
	}

	return destination, nil
}

func maskedColor(value, mask uint32) uint8 {
	if mask == 0 {
		return 0
	}
	shift := bits.TrailingZeros32(mask)
	maximum := mask >> uint(shift)
	component := (value & mask) >> uint(shift)
	return uint8((uint64(component)*255 + uint64(maximum)/2) / uint64(maximum))
}
