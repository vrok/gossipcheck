package server

import "sync"

// History remembers IDs of last N messages and can quickly tell
// if observed message has been already processed.
// It uses a ring buffer to evict old entries.
type History struct {
	ids, msgs *fifoCache

	mu sync.RWMutex
}

// NewHistory creates a new cache of historical messages.
// It has two ring buffers, one (bigger) that remebers only IDs, and one
// (smaller) that remembers full messages. The former is used to avoid
// processing the same message twice, and the latter is used for retransmissions
// in case some peer node requests them.
// Panics if idsSize < msgSize.
func NewHistory(idsSize, msgsSize int) *History {
	if idsSize < msgsSize {
		panic("IDs buffer has to be at least as big as the message buffer")
	}
	return &History{
		ids:  newFifoCache(idsSize),
		msgs: newFifoCache(msgsSize),
	}
}

type fifoCache struct {
	ring    []string
	set     map[string]interface{}
	size, i int
}

func newFifoCache(size int) *fifoCache {
	return &fifoCache{
		ring: make([]string, size),
		set:  make(map[string]interface{}, size),
		size: size,
	}
}

func (c *fifoCache) Get(id string) (interface{}, bool) {
	v, ok := c.set[id]
	return v, ok
}

func (c *fifoCache) Add(id string, v interface{}) {
	c.i = (c.i + 1) % c.size
	delete(c.set, c.ring[c.i])
	c.ring[c.i] = id
	c.set[id] = v
}

// Observe adds a message to the history object, if it hasn't been seen before.
// The result is true if it was seen before, false otherwise.
// Of course, if a message is so old that it has been already evicted from both
// buffers, it will be considered new, hence the buffers should be relatively
// big if there's a huge traffic in the cluster.
func (h *History) Observe(m *Message) (old bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	_, ok := h.ids.Get(m.ID)
	if !ok {
		h.ids.Add(m.ID, true)
		h.msgs.Add(m.ID, m)
		return false
	}
	return true
}

// MessageIDs returns IDs of all messages that are fully remembered.
func (h *History) MessageIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var ids []string
	for id := range h.msgs.set {
		ids = append(ids, id)
	}
	return ids
}

// GetMessages returns full messages from the history with given IDs.
// IDs that are missing are just skipped.
func (h *History) GetMessages(ids []string) []*Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var msgs []*Message
	for _, id := range ids {
		m, ok := h.msgs.Get(id)
		if !ok {
			// Oh well, it was probably evicted in the meanwhile.
			continue
		}

		msgs = append(msgs, m.(*Message))
	}
	return msgs
}

// MissingIDs returns those IDs from the given list that have never been seen,
// and their full messages should be requested from the peer that advertised them.
func (h *History) MissingIDs(advertised []string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var ids []string
	for _, id := range advertised {
		_, ok := h.ids.Get(id)
		if !ok {
			// The ID is not present even in the large IDs buffer,
			// so we assume that we have never seen it before.
			ids = append(ids, id)
		}
	}
	return ids
}
