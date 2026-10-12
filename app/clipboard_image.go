package main

import "errors"

var (
	errNoClipboardImage          = errors.New("clipboard does not contain an image")
	errClipboardImageUnsupported = errors.New("image clipboard access is not supported on this platform")
)
