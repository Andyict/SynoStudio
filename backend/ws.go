package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type wsClient struct {
	send  chan []byte
	album int64
	staff bool
}

// Hub fans out realtime events to album rooms and to all connected staff.
type Hub struct {
	mu    sync.Mutex
	rooms map[int64]map[*wsClient]bool
	staff map[*wsClient]bool
}

func newHub() *Hub {
	return &Hub{rooms: map[int64]map[*wsClient]bool{}, staff: map[*wsClient]bool{}}
}

func (h *Hub) add(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.album > 0 {
		if h.rooms[c.album] == nil {
			h.rooms[c.album] = map[*wsClient]bool{}
		}
		h.rooms[c.album][c] = true
	}
	if c.staff {
		h.staff[c] = true
	}
}

func (h *Hub) remove(c *wsClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r := h.rooms[c.album]; r != nil {
		delete(r, c)
		if len(r) == 0 {
			delete(h.rooms, c.album)
		}
	}
	delete(h.staff, c)
}

func deliver(c *wsClient, b []byte) {
	select {
	case c.send <- b:
	default: // slow client; drop rather than block everyone
	}
}

// Album broadcasts to everyone viewing the album, and also to staff dashboards.
func (h *Hub) Album(albumID int64, ev map[string]any) {
	ev["album_id"] = albumID
	b, _ := json.Marshal(ev)
	h.mu.Lock()
	defer h.mu.Unlock()
	sent := map[*wsClient]bool{}
	for c := range h.rooms[albumID] {
		deliver(c, b)
		sent[c] = true
	}
	for c := range h.staff {
		if !sent[c] && c.album == 0 {
			deliver(c, b)
		}
	}
}

// Staff broadcasts to every connected staff client.
func (h *Hub) Staff(ev map[string]any) {
	b, _ := json.Marshal(ev)
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.staff {
		deliver(c, b)
	}
}

func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, err := url.Parse(o)
	if err != nil {
		return false
	}
	for _, h := range []string{r.Host, r.Header.Get("X-Forwarded-Host")} {
		if h != "" && strings.EqualFold(u.Host, h) {
			return true
		}
	}
	// Behind a reverse proxy the Host header may be rewritten; compare host names without ports.
	host := strings.Split(r.Host, ":")[0]
	return strings.EqualFold(u.Hostname(), host)
}

// handleWS: /ws?album=ID&as=staff|customer. Staff without album gets the global feed.
func (a *App) handleWS(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "bad origin", 403)
		return
	}
	var p *Principal
	if r.URL.Query().Get("as") == "staff" {
		p = a.staffFromRequest(r)
	} else {
		p = a.customerFromRequest(r)
	}
	if p == nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	albumID, _ := strconv.ParseInt(r.URL.Query().Get("album"), 10, 64)
	if albumID > 0 {
		if _, err := a.albumFor(p, albumID); err != nil {
			http.Error(w, "forbidden", 403)
			return
		}
	} else if p.Type != "staff" {
		http.Error(w, "album required", 400)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true}) // origin checked above
	if err != nil {
		return
	}
	defer conn.CloseNow()

	c := &wsClient{send: make(chan []byte, 64), album: albumID, staff: p.Type == "staff"}
	a.hub.add(c)
	defer a.hub.remove(c)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	// Reader: we do not expect client messages, but must read to process pings/close.
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-c.send:
			wctx, wc := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Write(wctx, websocket.MessageText, b)
			wc()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, pc := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pctx)
			pc()
			if err != nil {
				return
			}
		}
	}
}
