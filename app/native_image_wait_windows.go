//go:build windows
// +build windows

package main

import "golang.org/x/sys/windows"

var procGetch = windows.NewLazySystemDLL("msvcrt.dll").NewProc("_getch")

func waitForNativeImageKey() {
	_, _, _ = procGetch.Call()
}
