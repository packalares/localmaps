// Package ws hosts an in-process WebSocket hub that fans server-side
// events (region.*, job.*) out to subscribed browser clients. The wire
// format is specified in docs/03-contracts.md and in
// contracts/openapi.yaml under /api/ws.
package ws

import (
	"encoding/json"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"

	"github.com/packalares/localmaps/server/internal/apierr"
)

// Event is the envelope the hub broadcasts to clients.
// `type` examples (from openapi.yaml):
//   region.progress, region.ready, region.failed
//   job.started, job.progress, job.complete, job.failed
type Event struct {
	Type    string `json:"type"`
	Channel string `json:"-"`    // hub-internal routing; not on wire
	Data    any    `json:"data"`
}

// clientMsg is the client→server subscribe/unsubscribe frame.
type clientMsg struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
}

// client represents one connected browser.
type client struct {
	conn     *websocket.Conn
	channels map[string]struct{}
	send     chan []byte
	mu       sync.Mutex
}

// Hub fans out Events to subscribed clients.
type Hub struct {
	mu       sync.RWMutex
	clients  map[*client]struct{}
	upgrader websocket.FastHTTPUpgrader
	inbound  chan Event
	stop     chan struct{}
}

// NewHub constructs a hub ready to accept clients via Handler().
func NewHub() *Hub {
	h := &Hub{
		clients: make(map[*client]struct{}),
		upgrader: websocket.FastHTTPUpgrader{
			// Allow any Origin — browser page is served from the same
			// gateway; cross-origin policy is enforced elsewhere.
			CheckOrigin: func(_ *fasthttp.RequestCtx) bool { return true },
		},
		inbound: make(chan Event, 256),
		stop:    make(chan struct{}),
	}
	go h.fanout()
	return h
}

// Publish enqueues an event for delivery to all subscribers of
// ev.Channel. If ev.Channel is empty, the hub falls back to ev.Type.
func (h *Hub) Publish(ev Event) {
	if ev.Channel == "" {
		ev.Channel = ev.Type
	}
	select {
	case h.inbound <- ev:
	default:
		// Drop when saturated; loss is acceptable for progress events.
	}
}

// Close stops the fan-out goroutine and closes all client connections.
func (h *Hub) Close() {
	close(h.stop)
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		_ = c.conn.Close()
		close(c.send)
	}
	h.clients = map[*client]struct{}{}
}

// Handler returns a Fiber handler that upgrades to a WebSocket when the
// request is an actual WS upgrade, else responds 400.
func (h *Hub) Handler() fiber.Handler {
	return func(c fiber.Ctx) error {
		if !websocket.FastHTTPIsWebSocketUpgrade(c.RequestCtx()) {
			return apierr.Write(c, apierr.CodeBadRequest,
				"expected WebSocket upgrade", false)
		}
		err := h.upgrader.Upgrade(c.RequestCtx(), func(conn *websocket.Conn) {
			h.serve(conn)
		})
		if err != nil {
			return err
		}
		return nil
	}
}

// serve runs the read + write loops for a newly-upgraded connection.
func (h *Hub) serve(conn *websocket.Conn) {
	cl := &client{
		conn:     conn,
		channels: make(map[string]struct{}),
		send:     make(chan []byte, 32),
	}
	h.mu.Lock()
	h.clients[cl] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, cl)
		h.mu.Unlock()
		_ = conn.Close()
	}()

	// Writer goroutine.
	go func() {
		for msg := range cl.send {
			_ = conn.WriteMessage(websocket.TextMessage, msg)
		}
	}()

	// Reader loop handles subscribe/unsubscribe frames.
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var m clientMsg
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		cl.mu.Lock()
		switch m.Type {
		case "subscribe":
			cl.channels[m.Channel] = struct{}{}
		case "unsubscribe":
			delete(cl.channels, m.Channel)
		}
		cl.mu.Unlock()
	}
}

// fanout delivers every inbound Event to every subscribed client.
func (h *Hub) fanout() {
	for {
		select {
		case <-h.stop:
			return
		case ev := <-h.inbound:
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			h.mu.RLock()
			for cl := range h.clients {
				cl.mu.Lock()
				_, subscribed := cl.channels[ev.Channel]
				cl.mu.Unlock()
				if !subscribed {
					continue
				}
				select {
				case cl.send <- b:
				default:
					// Client's buffer full; drop this event for them.
				}
			}
			h.mu.RUnlock()
		}
	}
}
