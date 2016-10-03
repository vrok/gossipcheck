package checks

import (
	"encoding/gob"
	"errors"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// CheckType represents the type of check to be performed.
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

// GobEncode is here to implement gob.GobEncoder. See the docs for CheckType to know why.
func (ct CheckType) GobEncode() ([]byte, error) {
	id, ok := typeToID[ct]
	if !ok {
		return nil, errors.New("Unexpected check")
	}
	return []byte{id}, nil
}

// GobDecode is here to implement gob.GobDecoder. See the docs for CheckType to know why.
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
}

// ParamsGroup describes a batch of checks to be performed.
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

// Checker is the interface whose implementations can run checks of their types.
type Checker interface {
	Type() CheckType
	Run(*Params) error
}

type fileExistsCheck struct{}

func (fe fileExistsCheck) Type() CheckType { return CheckFileExists }

func (fe fileExistsCheck) Run(p *Params) error {
	if _, err := os.Stat(p.Path); os.IsNotExist(err) {
		return err
	}
	return nil
}

type fileContainsCheck struct {
	// Batch size is modifiable for tests.
	batchMult int
}

func (fc fileContainsCheck) Type() CheckType { return CheckFileContains }

func (fc fileContainsCheck) Run(p *Params) error {
	f, err := os.Open(p.Path)
	if err != nil {
		return errors.New("Error opening file: " + err.Error())
	}
	defer f.Close()

	// Search file in batches. Adjacent batches have an overlap of len(sep) size.
	// TODO: Copy either Rabin-Karp or Boyer-Moore from Go's strings and make it work
	// with io.Reader (though this should be fast too).
	sep := p.Check

	batchMult := fc.batchMult
	if batchMult == 0 {
		batchMult = 2000
	}

	batchSize := len(sep) * batchMult
	buf := make([]byte, batchSize)

	start := 0

	for {
		n, err := f.Read(buf[start:])
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		end := start + n

		// Use the strings module within batches (it uses Rabin-Karp).
		if strings.Contains(string(buf[:end]), sep) {
			return nil
		}

		overlap := len(sep)
		if overlap > end {
			start = end
			continue
		}

		copy(buf[:overlap], buf[end-overlap:end])
		start = overlap
	}

	return errors.New("File doesn't contain given text")
}

type procRunningCheck struct{}

func (fc procRunningCheck) Type() CheckType { return CheckProcRunning }

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
	typeToCheck = make(map[CheckType]Checker)
	typeToID    = make(map[CheckType]byte)
	idToType    = make(map[byte]CheckType)
	maxID       byte
)

// AddCheck registers a checker. Don't use it after any node started working
// (init and tests initialisation are okay).
func AddCheck(ch Checker) {
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
	for _, ch := range []Checker{
		fileExistsCheck{},
		fileContainsCheck{},
		procRunningCheck{},
		checkEmpty{},
	} {
		AddCheck(ch)
	}
}

// GetCheck returns a check with given name (if it was registered).
func GetCheck(typ CheckType) (ch Checker, ok bool) {
	ch, ok = typeToCheck[typ]
	return
}
