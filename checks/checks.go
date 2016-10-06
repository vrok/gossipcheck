package checks

import (
	"encoding/gob"
	"errors"
	"log"
	"os/exec"
	"sync"
)

// Checker is the interface whose implementations run checks on nodes.
type Checker interface {
	// Type returns a unique value by which checks carried out by this checker
	// are identified.
	Type() CheckType
	// Run executes the check, returns nil if check succeeded. Otherwise,
	// it should return a non-nil error with information about the cause.
	Run(*Params) error
}

// GetCheck returns a checker with given type (if it was registered).
func GetCheck(typ CheckType) (ch Checker, ok bool) {
	mu.RLock()
	defer mu.RUnlock()

	ch, ok = typeToCheck[typ]
	return
}

// AddCheck registers a checker.
func AddCheck(ch Checker) {
	mu.Lock()
	defer mu.Unlock()

	gob.Register(ch)
	typeToCheck[ch.Type()] = ch
	typeToID[ch.Type()] = maxID
	idToType[maxID] = ch.Type()
	maxID++
}

// CheckType represents type of the check to be performed.
// There's a fixed number of check types, so there's an opportunity to save
// bandwidth in the gossip protocol by avoiding sending full type names,
// hence the custom gob encoding.
type CheckType string

// Pre-defined checks. It is possible to add other ones, e.g. tests do that.
const (
	CheckFileContains CheckType = "file_contains"
	CheckFileExists             = "file_exists"
	CheckProcRunning            = "process_running"
)

// Params describes one check. They can be sent in batches, see ParamsGroup.
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
	// Messages can be attached to checks when transferred on the wire.
	// They have no effect when put in JSON files.
	Message string
}

// ParamsGroup describes a batch of checks to be performed.
type ParamsGroup []*Params

// Run all checks from a group and return all non-nil errors.
func (pg ParamsGroup) Run() (errs map[string]error) {
	type nameErrPair struct {
		name string
		err  error
	}
	errs = make(map[string]error)

	errCh := make(chan *nameErrPair, len(pg))
	// Similar to /x/sync/errgroup, but collects all errors, not just one.
	var wg sync.WaitGroup
	for _, p := range pg {
		chk, ok := GetCheck(p.Type)
		if !ok {
			log.Print("Unknown check: " + p.Name)
			continue
		}
		wg.Add(1)
		go func(p *Params) {
			err := chk.Run(p)
			if err == nil {
				wg.Done()
				return
			}

			errCh <- &nameErrPair{p.Name, err}
			if p.Action != "" {
				cmd := exec.Command("sh", "-c", p.Action)
				output, err := cmd.CombinedOutput()
				log.Printf("Ran '%s' because check %s failed, output:\n%s\n",
					p.Action, p.Name, string(output))
				if err != nil {
					log.Printf("Running '%s' returned error: %s", p.Action, err)
				}
			}
			wg.Done()
		}(p)
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Collect results from all checks.
	for p := range errCh {
		errs[p.name] = p.err
	}
	return errs
}

func init() {
	// Register all checks. This is the only place where new checks have to be added.
	// WARNING: Changing order in which checks are added chacnges the way they are
	// serialized on the wire.
	for _, ch := range []Checker{
		fileExistsCheck{},
		fileContainsCheck{},
		procRunningCheck{},
		checkEmpty{},
	} {
		AddCheck(ch)
	}
}

var (
	typeToCheck = make(map[CheckType]Checker)
	typeToID    = make(map[CheckType]byte)
	idToType    = make(map[byte]CheckType)
	maxID       byte
	mu          sync.RWMutex
)

// GobEncode is here to implement gob.GobEncoder. See the docs for CheckType to know why.
func (ct CheckType) GobEncode() ([]byte, error) {
	mu.RLock()
	defer mu.RUnlock()

	id, ok := typeToID[ct]
	if !ok {
		return nil, errors.New("Unexpected check")
	}
	return []byte{id}, nil
}

// GobDecode is here to implement gob.GobDecoder. See the docs for CheckType to know why.
func (ct *CheckType) GobDecode(b []byte) error {
	mu.RLock()
	defer mu.RUnlock()

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
