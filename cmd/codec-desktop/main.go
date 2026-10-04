//go:build windows

// Command codec-desktop is codec in its own window. It serves the same
// handlers and UI as `codec serve` on a loopback port and points a
// Wails window at it, so every feature behaves as in the browser.
//
// A loopback server instead of Wails' own asset handler: on Windows
// that handler buffers each response until it ends, so the live file
// events (Server-Sent Events) could never arrive (decision 83).
//
// Usage: codec-desktop [folder | file]
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/version"
	"github.com/mahasenabheetha/codec/v2/internal/web"
)

// port is fixed so the window's origin, and with it the display
// preferences kept in local storage, stays the same between runs. v1
// uses 8765, scripts/dev.sh 8766 and the local launcher 8767.
const port = 8769

const (
	minWidth  = 720
	minHeight = 480

	defaultWidth  = 1400
	defaultHeight = 900
)

var (
	eventStarted = events.Common.ApplicationStarted
	eventClosing = events.Common.WindowClosing
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// desktop joins the window and the server: it is the server's view of
// the window (web.Desktop) and handles later launches.
type desktop struct {
	app      *application.App
	win      *application.WebviewWindow
	cfg      *config.Store
	server   atomic.Pointer[web.Server] // set once the server exists
	quitting atomic.Bool                // Quit from the tray: let the window close
}

func (d *desktop) SetTheme(dark bool) { setFrameTheme(d.win, dark) }

func (d *desktop) Settings() (web.DesktopSettings, error) {
	login, err := d.app.Autostart.IsEnabled()
	return web.DesktopSettings{StartAtLogin: login, KeepInTray: !d.cfg.Get().Desktop.QuitOnClose}, err
}

func (d *desktop) SetSettings(st web.DesktopSettings) error {
	if err := d.cfg.Update(func(s *config.Settings) { s.Desktop.QuitOnClose = !st.KeepInTray }); err != nil {
		return err
	}
	if login, err := d.app.Autostart.IsEnabled(); err != nil || login == st.StartAtLogin {
		return err
	}
	if st.StartAtLogin {
		// Registers this exe (per user, no admin) to start in the tray.
		return d.app.Autostart.EnableWithOptions(application.AutostartOptions{Arguments: []string{hiddenFlag}})
	}
	return d.app.Autostart.Disable()
}

func (d *desktop) PickFolder(title, start string) (string, error) {
	dlg := d.app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetTitle(title).
		AttachToWindow(d.win)
	if start != "" {
		dlg.SetDirectory(start)
	}
	path, err := dlg.PromptForSingleSelection()
	if err != nil && err.Error() == "cancelled by user" {
		// Wails' cancel error lives in an internal package; only its
		// text can be compared.
		return "", nil
	}
	return path, err
}

func run() error {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	d := &desktop{}
	hidden := slices.Contains(os.Args[1:], hiddenFlag)
	icon, _ := web.AppFile("icon-512.png")

	// New exits here when codec is already running, after handing this
	// launch's arguments to the running window.
	app := application.New(application.Options{
		Name:        "codec",
		Description: "The local workbench for a DevOps engineer's day",
		Icon:        icon,
		OnShutdown:  stop,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "io.github.mahasenabheetha.codec",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) { d.secondLaunch(data) },
		},
		Windows: application.WindowsOptions{
			// One stable folder for the window's storage, instead of
			// one named after the exe.
			WebviewUserDataPath: webviewDataDir(),
		},
	})
	d.app = app

	cfg, err := config.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	if cfg == nil {
		cfg = config.Memory()
	}
	d.cfg = cfg
	s := web.New(web.Options{Config: cfg, Desktop: d})
	d.server.Store(s)

	wd, _ := os.Getwd()
	req, hasReq := requestFor(os.Args[1:], wd)
	if hasReq {
		if err := s.OpenWorkspace(req.Root); err != nil {
			fmt.Fprintln(os.Stderr, "warning:", err)
			req = openRequest{}
		}
	}

	ln, err := listen()
	if err != nil {
		return err
	}
	served := make(chan error, 1)
	go func() { served <- s.Serve(ctx, ln) }()

	d.win = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "codec " + version.Get().Version,
		Width:            defaultWidth,
		Height:           defaultHeight,
		MinWidth:         minWidth,
		MinHeight:        minHeight,
		URL:              "http://" + ln.Addr().String() + "/" + req.route(),
		BackgroundColour: application.NewRGB(0x0b, 0x0d, 0x12), // --bg-0, no white flash
		Hidden:           hidden,
	})
	rememberWindow(app, d.win, hidden)
	d.keepInTray()
	d.setupTray()

	runErr := app.Run()
	stop()
	if err := <-served; err != nil && !errors.Is(err, context.Canceled) {
		return errors.Join(runErr, err)
	}
	return runErr
}

// secondLaunch brings the window forward and opens what the second
// launch was given.
func (d *desktop) secondLaunch(data application.SecondInstanceData) {
	args := data.Args
	if len(args) > 0 {
		args = args[1:] // the program itself
	}
	if req, ok := requestFor(args, data.WorkingDir); ok && d.server.Load() != nil {
		// Through the server's event stream, not ExecJS: Wails runs
		// scripts only once its own frontend library says it is ready,
		// and codec's page doesn't load it.
		if err := d.server.Load().OpenPath(req.Root, req.File); err != nil {
			fmt.Fprintln(os.Stderr, "warning:", err)
		}
	}
	d.showWindow()
}

// webviewDataDir is where WebView2 keeps the window's storage (local
// storage with the display preferences, cache), next to codec's other
// local data.
func webviewDataDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "codec", "webview")
}

// listen takes the fixed port, or any free one when it is taken (the
// app still works; display preferences start fresh for that run).
func listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err == nil {
		return ln, nil
	}
	fmt.Fprintf(os.Stderr, "warning: port %d is taken, using a free one: %v\n", port, err)
	return net.Listen("tcp", "127.0.0.1:0")
}
