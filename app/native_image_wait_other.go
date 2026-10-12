//go:build !windows
// +build !windows

package main

import (
	"bufio"
	"os"
)

func waitForNativeImageKey() {
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
