package server

import "gossipcheck/checks"

type MsgType int

const (
	// Run checks once.
	RunChecks MsgType = iota
	// Node can tell other node about messages it remembers.
	AdvertiseMsgs
	// Node can requests missing messages from another node.
	ReqestMsgs
	// Install checks to be running continually.
	InstallChecks
	// Delete installed checks.
	DeleteChecks
	// Feedback message about a failed check.
	CheckFailed
)

type Message struct {
	Type MsgType
	ID   string
	// Node where the message originated.
	OrigNode string
	// Node that (re)sent this message previously.
	SrcNode string
	// For requests that don't carry full definitions (likeDeleteChecks and
	// CheckFailed), only Params.Name is filled, the rest of the fields are
	// set to zero values (they are skipped in gobs).
	Params checks.ParamsGroup
	// Used by AdvertiseMsgs and ReqestMsgs.
	MessageIDs []string
}
