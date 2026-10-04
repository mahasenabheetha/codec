// Command codec-desktop is codec in its own window. It serves the same
// handlers and UI as `codec serve` on a loopback port and points a
// Wails window at it, so every feature behaves as in the browser.
//
// A loopback server instead of Wails' own asset handler: on Windows
// that handler buffers each response until it ends, so the live file
// events (Server-Sent Events) could never arrive.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/version"
	"github.com/mahasenabheetha/codec/v2/internal/web"
)

// port is fixed so the window's origin, and with it the display
// preferences kept in local storage, stays the same between runs. v1
// uses 8765, scripts/dev.sh 8766 and the local launcher 8767.
const port = 8769

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	s := web.New(web.Options{Config: cfg})

	ln, err := listen()
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/"

	ctx, stop := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- s.Serve(ctx, ln) }()

	app := application.New(application.Options{
		Name:        "codec",
		Description: "The local workbench for a DevOps engineer's day",
		OnShutdown:  stop,
		Windows:     application.WindowsOptions{},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "codec " + version.Get().Version,
		Width:            1400,
		Height:           900,
		MinWidth:         720,
		MinHeight:        480,
		URL:              url,
		BackgroundColour: application.NewRGB(0x0b, 0x0d, 0x12), // --bg-0, no white flash
	})

	runErr := app.Run()
	stop()
	if err := <-served; err != nil && !errors.Is(err, context.Canceled) {
		return errors.Join(runErr, err)
	}
	return runErr
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
