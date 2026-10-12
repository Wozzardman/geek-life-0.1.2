//go:build !windows
// +build !windows

package main

import "image"

func readClipboardImage() (image.Image, error) {
	return nil, errClipboardImageUnsupported
}
