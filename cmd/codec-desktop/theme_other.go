//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// setFrameTheme: macOS and Linux follow the system appearance for now.
func setFrameTheme(*application.WebviewWindow, bool) {}
