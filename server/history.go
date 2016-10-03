package server

import "sync"

// History remembers IDs of last N messages and can quickly tell
// if observed message has been already processed.
// It uses a ring buffer to evict old entries.
type History struct {
	ids, msgs *fifoCache

	mu sync.RWMutex
}

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

// IDs of all messages that are remembered.
func (h *History) MessageIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var ids []string
	for id := range h.msgs.set {
		ids = append(ids, id)
	}
	return ids
}

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

// Given a list of message IDs, returns those from the list that
// we have never seen (it means that we check if IDs are present
// in the large IDs cyclic buffer).
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
