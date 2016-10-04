package server

import "github.com/vrok/gossipcheck/checks"

// MsgType represents the message type used in the checks gossip protocol.
type MsgType int

//go:generate stringer -type=MsgType

const (
	// RunChecks message tells nodes to run checks once.
	RunChecks MsgType = iota
	// AdvertiseMsgs tells nodes about messages it remembers.
	// It is possible that a small number of nodes will miss a message
	// in the initial burst phase (especially when the gossip group is
	// small), thanks to advertising it will always eventually converge.
	AdvertiseMsgs
	// RequestMsgs is sent in response to AdvertiseMsgs.
	// This way, a node can request missing messages from another node.
	RequestMsgs
)

// Message represents a message that is transmitted in gobs between nodes.
// Either Params or MessageIDs are non-nil, depending on message type.
type Message struct {
	Type MsgType
	ID   string
	// Node where the message originated.
	OrigNode string
	// Node that (re)sent this message previously.
	SrcNode string
	// Definitions of checks.
	Params checks.ParamsGroup
	// Used by AdvertiseMsgs and ReqestMsgs.
	MessageIDs []string
}

// IsOneOff tells whether a message is just a one-off message, that shouldn't
// be remembered in the history circular buffer.
func (m *Message) IsOneOff() bool {
	return m.Type == AdvertiseMsgs || m.Type == RequestMsgs
}
