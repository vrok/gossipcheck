package server

import "sync"

// History remembers IDs of last N messages and can quickly tell
// if observed message has been already processed.
// It uses a ring buffer to evict old entries.
type History struct {
	ring    []string
	set     map[string]struct{}
	size, i int
	mu      sync.Mutex
}

func NewHistory(size int) *History {
	return &History{
		ring: make([]string, size),
		set:  make(map[string]struct{}, size),
		size: size,
	}
}

func (h *History) Observe(id string) (old bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, old = h.set[id]
	if !old {
		h.i = (h.i + 1) % h.size
		delete(h.set, h.ring[h.i])
		h.ring[h.i] = id
		h.set[id] = struct{}{}
	}
	return
}
