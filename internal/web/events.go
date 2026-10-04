package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// event is one Server-Sent Event: a name and a JSON payload.
type event struct {
	name string
	data any
}

// hub fans events out to every connected browser tab.
type hub struct {
	mu   sync.Mutex
	subs map[chan event]struct{}
	held *event // see publishOrHold
}

func newHub() *hub { return &hub{subs: map[chan event]struct{}{}} }

// subscribe returns a channel of events and a function that ends the
// subscription.
func (h *hub) subscribe() (<-chan event, func()) {
	ch := make(chan event, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	if h.held != nil {
		ch <- *h.held
		h.held = nil
	}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

// publish sends e to every subscriber without blocking: a stalled tab
// misses events rather than stalling the watcher. (Its next tree
// refresh catches it up.)
func (h *hub) publish(name string, data any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- event{name, data}:
		default:
		}
	}
}

// publishOrHold is publish for a request that must not be lost: with no
// tab listening (the page is still loading, or reconnecting), the last
// such event is kept for the next subscriber.
func (h *hub) publishOrHold(name string, data any) {
	h.mu.Lock()
	if len(h.subs) == 0 {
		h.held = &event{name, data}
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	h.publish(name, data)
}

// heartbeat keeps idle connections open through proxies and lets the
// browser notice a dead server.
const heartbeat = 25 * time.Second

// handleEvents streams events as text/event-stream until the client
// goes away or the server shuts down (both cancel r.Context()).
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")

	events, unsubscribe := s.hub.subscribe()
	defer unsubscribe()

	// "retry" tells EventSource how soon to reconnect after a drop.
	fmt.Fprint(w, "retry: 2000\n\n")
	if err := rc.Flush(); err != nil {
		return
	}

	tick := time.NewTicker(heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n") // a comment line: ignored by the browser
		case e := <-events:
			data, err := json.Marshal(e.data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.name, data)
		}
		if err := rc.Flush(); err != nil {
			return
		}
	}
}
