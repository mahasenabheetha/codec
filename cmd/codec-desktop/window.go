package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/mahasenabheetha/codec/v2/internal/config"
)

// windowState is where the window was when it closed, in
// scale-independent units (Wails' DIP), kept in codec's settings folder
// next to settings.json.
type windowState struct {
	X, Y, Width, Height int
	Maximised           bool
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
	_ = os.WriteFile(path, b, 0o600) // best effort: next start falls back to the default size
}

// onScreen reports whether enough of r lies on some screen's work area
// to grab the window again (a monitor may have been unplugged).
func onScreen(app *application.App, r application.Rect) bool {
	for _, s := range app.Screen.GetAll() {
		wa := s.WorkArea
		w := min(r.X+r.Width, wa.X+wa.Width) - max(r.X, wa.X)
		h := min(r.Y+r.Height, wa.Y+wa.Height) - max(r.Y, wa.Y)
		if w >= 120 && h >= 60 {
			return true
		}
	}
	return false
}

// fitDefault centres the first window on the primary screen at the
// default size, or 90% of the work area when that is smaller (a
// laptop at 150% scaling has less room than 1400×900).
func fitDefault(app *application.App, win *application.WebviewWindow) {
	s := app.Screen.GetPrimary()
	if s == nil {
		return
	}
	wa := s.WorkArea
	w := min(defaultWidth, wa.Width*9/10)
	h := min(defaultHeight, wa.Height*9/10)
	win.SetBounds(application.Rect{X: wa.X + (wa.Width-w)/2, Y: wa.Y + (wa.Height-h)/2, Width: w, Height: h})
}

// rememberWindow restores the last bounds before the window first
// shows (Wails keeps it hidden until the page has loaded) and saves
// them when it closes. While maximised, the normal bounds from before
// are kept, so un-maximising next time returns there.
func rememberWindow(app *application.App, win *application.WebviewWindow, hidden bool) {
	st, ok := loadState()
	app.Event.OnApplicationEvent(eventStarted, func(*application.ApplicationEvent) {
		r := application.Rect{X: st.X, Y: st.Y, Width: st.Width, Height: st.Height}
		if !ok || !onScreen(app, r) {
			fitDefault(app, win)
			return
		}
		win.SetBounds(r)
		if st.Maximised && !hidden { // maximising would show a window started in the tray
			win.Maximise()
		}
	})
	win.RegisterHook(eventClosing, func(*application.WindowEvent) {
		next := st
		next.Maximised = win.IsMaximised()
		if !next.Maximised && !win.IsMinimised() {
			b := win.Bounds()
			next.X, next.Y, next.Width, next.Height = b.X, b.Y, b.Width, b.Height
		}
		if next.Width >= minWidth && next.Height >= minHeight {
			saveState(next)
		}
	})
}
