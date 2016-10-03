package checks

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
)

type CheckType string

const (
	CheckFileContains CheckType = "file_contains"
	CheckFileExists             = "file_exists"
	CheckProcRunning            = "proc_running"
)

// There's a fixed number of check types, so there's an opportunity to save
// bandwidth in the gossip protocol by avoiding sending full type names.
func (ct CheckType) GobEncode() ([]byte, error) {
	id, ok := typeToID[ct]
	if !ok {
		return nil, errors.New("Unexpected check")
	}
	return []byte{id}, nil
}

func (ct *CheckType) GobDecode(b []byte) error {
	if len(b) != 1 {
		return errors.New("Bad lengh")
	}
	typ, ok := idToType[b[0]]
	if !ok {
		return errors.New("Unexpected byte")
	}
	*ct = typ
	return nil
}

type Params struct {
	// Name of the check, should be unique.
	Name string
	// Type of the check.
	Type CheckType
	// For file checks, contains a path to an arbitrary file.
	// For process checks, can be empty (then it's ignored) or point to an executable file.
	Path string
	// Value that has to be present either in the checked file or in arguments of the checked
	// process.
	Check string
	// Command that will be run if this check fails.
	Action string
}

type ParamsGroup []*Params

// Run all checks from a group and return all non-nil errors.
func (pg ParamsGroup) Run() (errs []error) {
	errCh := make(chan error, len(pg))
	// Similar to /x/sync/errgroup, but collects all errors, not just one.
	var wg sync.WaitGroup
	for _, p := range pg {
		chk, ok := GetCheck(p.Type)
		if !ok {
			errs = append(errs, errors.New("Unknown check: "+p.Name))
			continue
		}
		wg.Add(1)
		go func(p *Params) {
			err := chk.Run(p)
			if err != nil {
				errCh <- err
			}
			wg.Done()
		}(p)
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Collect results from all checks.
	for err := range errCh {
		errs = append(errs, err)
	}
	return errs
}

type Check interface {
	Type() CheckType
	Run(*Params) error
}

type fileExistsCheck struct{}

func (fe fileExistsCheck) Type() CheckType { return "file_exists" }

func (fe fileExistsCheck) Run(p *Params) error {
	if _, err := os.Stat(p.Path); err != nil {
		return fmt.Errorf("File '%s' doesn't exist", p.Path)
	}
	return nil
}

type fileContainsCheck struct{}

func (fc fileContainsCheck) Type() CheckType { return "file_contains" }

func (fc fileContainsCheck) Run(p *Params) error {
	log.Println("file contains check")
	return nil
}

type procRunningCheck struct{}

func (fc procRunningCheck) Type() CheckType { return "process_running" }

func (fc procRunningCheck) Run(p *Params) error {
	log.Println("proc running check")
	return nil
}

// Useful for testing. Always succeeds when check is empty, fails otherwise.
type checkEmpty struct{}

func (fe checkEmpty) Type() CheckType { return "check_empty" }

func (fe checkEmpty) Run(p *Params) error {
	if p.Check != "" {
		return errors.New("Check is not empty")
	}
	return nil
}

var (
	typeToCheck = make(map[CheckType]Check)
	typeToID    = make(map[CheckType]byte)
	idToType    = make(map[byte]CheckType)
	maxID       byte
)

// Don't add checks in places other than init() and tests initialisation.
func AddCheck(ch Check) {
	gob.Register(ch)
	typeToCheck[ch.Type()] = ch
	typeToID[ch.Type()] = maxID
	idToType[maxID] = ch.Type()
	maxID++
}

func init() {
	// Register all checks. This is the only place where new checks have to be added.
	// WARNING: Changing order in which checks are added chacnges the way they are
	// serialized on the wire.
	for _, ch := range []Check{
		fileExistsCheck{},
		fileContainsCheck{},
		procRunningCheck{},
		checkEmpty{},
	} {
		AddCheck(ch)
	}
}

// Return a check with given name.
func GetCheck(typ CheckType) (ch Check, ok bool) {
	ch, ok = typeToCheck[typ]
	return
}
