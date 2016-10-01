package checks

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"os"
)

type CheckType string

const (
	CheckFileContains CheckType = "file_contains"
	CheckFileExists             = "file_exists"
	CheckProcRunning            = "proc_running"
)

// Converve bytes in the gossip protocol.
func (ct CheckType) GobEncode() ([]byte, error) {
	switch ct {
	case CheckFileContains:
		return []byte{0}, nil
	case CheckFileExists:
		return []byte{1}, nil
	case CheckProcRunning:
		return []byte{2}, nil
	}
	return nil, errors.New("Unexpected check")
}

// Converve bytes in the gossip protocol.
func (ct *CheckType) GobDecode(b []byte) error {
	if len(b) != 1 {
		return errors.New("Bad lengh")
	}
	switch b[0] {
	case 0:
		*ct = CheckFileContains
	case 1:
		*ct = CheckFileExists
	case 2:
		*ct = CheckProcRunning
	default:
		return errors.New("Unexpected byte")
	}
	return nil
}

type Params struct {
	// Name of the check, should be unique.
	Name string
	// Type of the check.
	Type string
	// For file checks, contains a path to an arbitrary file.
	// For process checks, can be empty (then it's ignored) or point to an executable file.
	Path string
	// Value that has to be present either in the checked file or in arguments of the checked
	// process.
	Check string
	// Command that will be run if this check fails.
	Action string
}

type Check interface {
	Type() string
	Run(*Params) error
}

type fileExistsCheck struct{}

func (fe fileExistsCheck) Type() string { return "file_exists" }

func (fe fileExistsCheck) Run(p *Params) error {
	if _, err := os.Stat(p.Path); err != nil {
		return fmt.Errorf("File '%s' doesn't exist", p.Path)
	}
	return nil
}

type fileContainsCheck struct{}

func (fc fileContainsCheck) Type() string { return "file_contains" }

func (fc fileContainsCheck) Run(p *Params) error {
	log.Println("file contains check")
	return nil
}

type procRunningCheck struct{}

func (fc procRunningCheck) Type() string { return "process_running" }

func (fc procRunningCheck) Run(p *Params) error {
	log.Println("proc running check")
	return nil
}

// Useful for testing. Always succeeds when check is empty, fails otherwise.
type checkEmpty struct{}

func (fe checkEmpty) Type() string { return "check_empty" }

func (fe checkEmpty) Run(p *Params) error {
	if p.Check != "" {
		return errors.New("Check is not empty")
	}
	return nil
}

var nameToCheck map[string]Check

func init() {
	nameToCheck = make(map[string]Check)

	// Register all checks. This is the only place where new checks have to be added.
	for _, ch := range []Check{
		fileExistsCheck{},
		fileContainsCheck{},
		procRunningCheck{},
		checkEmpty{},
	} {
		gob.Register(ch)
		nameToCheck[ch.Type()] = ch
	}
}

// Return a check with given name.
func GetCheck(name string) (ch Check, ok bool) {
	ch, ok = nameToCheck[name]
	return
}
