package workspace

import (
	"context"
	"errors"
	"log"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Change ops.
const (
	Added   = "added"
	Changed = "changed"
	Removed = "removed"
)

// Change is one file that appeared, changed or disappeared.
type Change struct {
	Op   string `json:"op"`
	Path string `json:"path"`
}

// Watch modes.
const (
	WatchNative = "native"
	WatchPoll   = "poll"
)

// Tuning; variables so tests can shorten them.
var (
	// debounce groups a burst of events (a git checkout, an editor's
	// save-via-rename, OneDrive's sync chatter) into one update.
	debounce = 200 * time.Millisecond
	// maxWait flushes anyway during a never-ending burst.
	maxWait = time.Second
	// pollInterval is how often polling mode rescans the folder.
	pollInterval = time.Second
)

// Watch scans the folder, then reports changes to emit until ctx is
// cancelled. It returns once the scan is done (the tree is ready);
// watching starts in the background because adding thousands of
// directory watches takes a while (~0.4 s for 2k directories on
// Windows). It uses native OS events unless poll is set or they can't
// be set up (too many directories, unsupported filesystem), in which
// case it polls. mode is told which one is in use.
//
// Native events only say "something happened at this path"; the
// workspace then rescans that path and diffs it against its index, so
// both modes report changes the same way.
func (w *Workspace) Watch(ctx context.Context, poll bool, emit func([]Change), mode func(string)) error {
	_, dirs, err := w.rescan(ctx, "")
	if err != nil {
		return err
	}
	go func() {
		if !poll {
			fw, err := w.nativeWatcher(ctx, dirs)
			if err == nil {
				mode(WatchNative)
				// Catch anything that changed while the watches were added.
				if changes, _, err := w.rescan(ctx, ""); err == nil && len(changes) > 0 {
					w.Classify(ctx)
					emit(changes)
				}
				w.watchNative(ctx, fw, emit)
				return
			}
			if ctx.Err() != nil {
				return
			}
			log.Printf("native file watching unavailable (%v); polling every %s instead", err, pollInterval)
		}
		mode(WatchPoll)
		w.poll(ctx, emit)
	}()
	return nil
}

// nativeWatcher watches every directory in the tree. fsnotify is not
// recursive, so each directory gets its own watch; new directories are
// added as they appear.
func (w *Workspace) nativeWatcher(ctx context.Context, dirs []string) (*fsnotify.Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		err := ctx.Err() // the folder was closed meanwhile
		if err == nil {
			err = fw.Add(w.abs(d))
		}
		if err != nil {
			fw.Close()
			return nil, err
		}
	}
	return fw, nil
}

func (w *Workspace) watchNative(ctx context.Context, fw *fsnotify.Watcher, emit func([]Change)) {
	defer fw.Close()

	pending := map[string]bool{}
	var first time.Time // when the current burst started
	timer := time.NewTimer(time.Hour)
	timer.Stop()

	schedule := func() {
		if first.IsZero() {
			first = time.Now()
		}
		// Wait for a quiet gap, but never longer than maxWait in total.
		timer.Reset(max(0, min(debounce, maxWait-time.Since(first))))
	}

	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-fw.Events:
			if !ok {
				return
			}
			p, ok := w.rel(ev.Name)
			if !ok || p == "" || w.excluded(p) {
				continue
			}
			pending[p] = true
			schedule()

		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				pending[""] = true // events were lost: rescan everything
				schedule()
			}

		case <-timer.C:
			first = time.Time{}
			var all []Change
			for _, p := range collapse(pending) {
				// A changed .gitignore can hide or reveal a whole subtree.
				if path.Base(p) == ".gitignore" {
					p = parentDir(p)
				}
				changes, dirs, err := w.rescan(ctx, p)
				if err != nil {
					continue
				}
				for _, d := range dirs {
					_ = fw.Add(w.abs(d)) // no-op for directories already watched
				}
				all = append(all, changes...)
			}
			clear(pending)
			if len(all) > 0 {
				w.Classify(ctx) // just the changed files; fast
				emit(all)
			}
		}
	}
}

// poll rescans the whole folder every pollInterval. It is the fallback
// for Docker bind mounts and network drives, where native events are
// missing or unreliable.
func (w *Workspace) poll(ctx context.Context, emit func([]Change)) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if changes, _, err := w.rescan(ctx, ""); err == nil && len(changes) > 0 {
				w.Classify(ctx)
				emit(changes)
			}
		}
	}
}

// collapse returns the pending paths without those inside another
// pending path, since rescanning a directory covers its contents.
func collapse(pending map[string]bool) []string {
	paths := make([]string, 0, len(pending))
	for p := range pending {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	var out []string
	for _, p := range paths {
		// Parents sort before their children, so checking the kept
		// paths is enough ("a-b" may sort between "a" and "a/c").
		if !underAny(p, out) {
			out = append(out, p)
		}
	}
	return out
}

// abs converts a slash path inside the workspace to an OS path.
func (w *Workspace) abs(p string) string {
	return filepath.Join(w.root, filepath.FromSlash(p))
}

// rel converts an OS path from an event back to a slash path.
func (w *Workspace) rel(name string) (string, bool) {
	r, err := filepath.Rel(w.root, name)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", false
	}
	if r == "." {
		return "", true
	}
	return filepath.ToSlash(r), true
}
