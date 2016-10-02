package server

import "gossipcheck/checks"

type MsgType int

const (
	// Run checks once.
	RunChecks MsgType = iota
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
	// For DeleteChecks and CheckFailed, only the Params.Name is filled,
	// the rest of the fields are set to zero values (they are skipped
	// in gobs).
	Params []*checks.Params
}
