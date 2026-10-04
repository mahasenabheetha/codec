//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/w32"

	"github.com/mahasenabheetha/codec/v2/internal/config"
)

// windowState is where the window was when it last closed: X, Y, Width
// and Height in scale-independent units (Wails' DIP), used to place it;
// the physical rectangle only to check it is still on a monitor.
type windowState struct {
	X, Y, Width, Height int
	Maximised           bool
	Physical            w32.RECT
}

func statePath() string {
	dir, err := config.Dir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "desktop-window.json")
}

func loadState() (windowState, bool) {
	var st windowState
	b, err := os.ReadFile(statePath())
	if err != nil || json.Unmarshal(b, &st) != nil || st.Width < minWidth || st.Height < minHeight {
		return st, false
	}
	return st, true
}

func saveState(st windowState) {
	path := statePath()
	if path == "" {
		return
	}
	b, _ := json.Marshal(st)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		logf("save window position: %v", err) // next start falls back to the default size
	}
}

// onMonitor reports whether r (physical pixels) overlaps a connected
// monitor; one may have been unplugged since.
func onMonitor(r w32.RECT) bool {
	return w32.MonitorFromRect(&r, w32.MONITOR_DEFAULTTONULL) != 0
}

// placeWindow sets the window's first position before it exists, so
// nothing has to move it afterwards: the saved bounds when they are
// still on a monitor, else the default size centred, shrunk to 90% of
// the primary work area when that is smaller (a laptop at 150% scaling
// has less room than 1400×900).
func placeWindow(opts *application.WebviewWindowOptions, st windowState, ok, hidden bool) {
	if ok && onMonitor(st.Physical) {
		opts.InitialPosition = application.WindowXY
		opts.X, opts.Y, opts.Width, opts.Height = st.X, st.Y, st.Width, st.Height
		if st.Maximised && !hidden { // maximising would show a window started in the tray
			opts.StartState = application.WindowStateMaximised
		}
		return
	}
	opts.InitialPosition = application.WindowCentered
	opts.Width, opts.Height = defaultWidth, defaultHeight
	m := w32.MonitorFromPoint(0, 0, w32.MONITOR_DEFAULTTOPRIMARY)
	var info w32.MONITORINFO
	info.CbSize = uint32(unsafe.Sizeof(info))
	var dpiX, dpiY w32.UINT
	if !w32.GetMonitorInfo(m, &info) || w32.GetDPIForMonitor(m, w32.MDT_EFFECTIVE_DPI, &dpiX, &dpiY) != 0 || dpiX == 0 {
		return
	}
	scale := float64(dpiX) / 96
	waW := int(float64(info.RcWork.Right-info.RcWork.Left) / scale)
	waH := int(float64(info.RcWork.Bottom-info.RcWork.Top) / scale)
	opts.Width = min(defaultWidth, waW*9/10)
	opts.Height = min(defaultHeight, waH*9/10)
}

// windowSaver saves the window's bounds. It runs when the window is
// closed to the tray and when codec quits; while maximised (or
// minimised) the last normal bounds are kept, so un-maximising next
// time returns there.
type windowSaver struct {
	win *application.WebviewWindow
	mu  sync.Mutex
	st  windowState
}

func (ws *windowSaver) save() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if !ws.win.IsVisible() {
		return // in the tray: saved when it was hidden, or never shown
	}
	next := ws.st
	next.Maximised = ws.win.IsMaximised()
	if !next.Maximised && !ws.win.IsMinimised() {
		b, p := ws.win.Bounds(), ws.win.PhysicalBounds()
		next.X, next.Y, next.Width, next.Height = b.X, b.Y, b.Width, b.Height
		next.Physical = w32.RECT{Left: int32(p.X), Top: int32(p.Y), Right: int32(p.X + p.Width), Bottom: int32(p.Y + p.Height)}
	}
	if next.Width >= minWidth && next.Height >= minHeight {
		ws.st = next
		saveState(next)
	}
}
