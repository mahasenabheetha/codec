package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"
)

// setFrameTheme darkens or lightens the title bar and native popup
// menus. Wails sets this only when the window is created; the page's
// theme can change at any time.
func setFrameTheme(win *application.WebviewWindow, dark bool) {
	application.InvokeAsync(func() {
		if h := win.NativeWindow(); h != nil {
			w32.SetTheme(uintptr(h), dark)
		}
	})
}
