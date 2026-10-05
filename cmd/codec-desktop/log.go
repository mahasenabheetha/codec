//go:build windows

package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/w32"
)

// A window app has no console, so problems go to desktop.log in codec's
// local folder (next to the sample and the window's storage), kept under
// maxLog by starting over when it grows past it.
const maxLog = 1 << 20

var logOut io.Writer = os.Stderr

func openLog() {
	base, err := os.UserCacheDir()
	if err != nil {
		return
	}
	path := filepath.Join(base, "codec", "desktop.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if info, err := os.Stat(path); err == nil && info.Size() > maxLog {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return
	}
	logOut = io.MultiWriter(f, os.Stderr)
	log.SetOutput(logOut)
}

// logf records something that went wrong but didn't stop codec.
func logf(format string, args ...any) { log.Printf(format, args...) }

// wailsLogger sends Wails' own messages (warnings and errors) to the
// same log.
func wailsLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(logOut, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

// fatal reports a failed start in a message box, since nothing else
// would show it, and exits.
func fatal(err error) {
	log.Printf("codec could not start: %v", err)
	w32.MessageBox(0, fmt.Sprintf("codec could not start:\n\n%v\n\nDetails are in desktop.log in %%LOCALAPPDATA%%\\codec.", err), "codec", w32.MB_OK|w32.MB_ICONERROR)
	os.Exit(1)
}
