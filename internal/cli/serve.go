package cli

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/version"
	"github.com/mahasenabheetha/codec/v2/internal/web"
)

var (
	servePort int
	serveHost string
	serveRoot string
	servePoll bool
	serveOpen bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the local web UI",
	Long: `serve starts a small local web server and hosts the codec UI:
the encode/decode tools and the read-only YAML workbench.

It binds to 127.0.0.1 only unless --host says otherwise, so nothing on
your network can reach it. codec never writes to the folders it opens.

Examples:
  codec serve --open                  start and open the browser
  codec serve --root ~/repos/app      open a folder right away
  codec serve --host 0.0.0.0 --root /work    inside Docker`,
	Args: cobra.NoArgs,
	RunE: runServe,
}

func init() {
	f := serveCmd.Flags()
	f.IntVar(&servePort, "port", 8765, "port to listen on")
	f.StringVar(&serveHost, "host", "127.0.0.1", "address to bind (0.0.0.0 only inside a container)")
	f.StringVar(&serveRoot, "root", "", "folder to open at start")
	f.BoolVar(&servePoll, "poll", false, "detect file changes by polling (automatic in containers)")
	f.BoolVar(&serveOpen, "open", false, "open the UI in the default browser")
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.Open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}

	// Native file events don't cross Docker bind mounts reliably
	// (especially from Windows hosts), so containers poll.
	poll := servePoll || inContainer()
	s := web.New(web.Options{Config: cfg, Poll: poll, Hosts: extraHosts(serveHost)})

	if serveRoot != "" {
		if err := s.OpenWorkspace(serveRoot); err != nil {
			return fmt.Errorf("--root: %w", err)
		}
	}

	ln, err := net.Listen("tcp", net.JoinHostPort(serveHost, strconv.Itoa(servePort)))
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) || isAddrInUse(err) {
			return fmt.Errorf("port %d is already in use (is codec already running?); try --port %d", servePort, servePort+1)
		}
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	url := "http://" + net.JoinHostPort(browserHost(serveHost), strconv.Itoa(port)) + "/"

	fmt.Printf("codec %s running at %s — Ctrl+C to stop\n", version.Get().Version, url)
	if inContainer() {
		// The container can't know which host port -p mapped it to.
		fmt.Printf("(in a container: open the host port you published, e.g. -p 127.0.0.1:%d:%d)\n", port, port)
	}
	if serveRoot != "" {
		mode := "native events"
		if poll {
			mode = "polling"
		}
		fmt.Printf("workspace: %s (watching with %s)\n", serveRoot, mode)
	}
	if serveOpen {
		if err := openBrowser(url); err != nil {
			fmt.Fprintln(os.Stderr, "could not open a browser:", err)
		}
	}

	// Ctrl+C (and docker stop's SIGTERM) cancel ctx; Serve then shuts
	// down cleanly, ending open event streams.
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return s.Serve(ctx, ln)
}

// extraHosts returns the bind address as an accepted Host name when it
// is a specific address (not a wildcard or loopback, which are always
// accepted).
func extraHosts(host string) []string {
	switch host {
	case "", "0.0.0.0", "::", "127.0.0.1", "localhost", "::1":
		return nil
	}
	return []string{host}
}

// browserHost is the address to put in the printed URL.
func browserHost(host string) string {
	if host == "0.0.0.0" || host == "::" || host == "" {
		return "localhost"
	}
	return host
}

// inContainer reports whether codec runs inside Docker or Podman.
func inContainer() bool {
	for _, f := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(f); err == nil {
			return true
		}
	}
	return false
}

// isAddrInUse catches Windows' WSAEADDRINUSE, which isn't EADDRINUSE.
func isAddrInUse(err error) bool {
	var se syscall.Errno
	return errors.As(err, &se) && runtime.GOOS == "windows" && se == 10048
}

// openBrowser opens url in the default browser without waiting for it.
func openBrowser(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		c = exec.Command("open", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	return c.Start()
}
