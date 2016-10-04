package checks

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
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
			if err != nil {
				errCh <- &nameErrPair{p.Name, err}
			}
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

var num = regexp.MustCompile("^\\d+$")

func fetchProcessExeAndArgs(pid string) (exe string, args string, err error) {
	exe, err = os.Readlink(fmt.Sprintf("/proc/%s/exe", pid))
	if err != nil {
		return "", "", fmt.Errorf("Couldn't read link for pid %d: %s", pid, err)
	}

	raw, err := ioutil.ReadFile(fmt.Sprintf("/proc/%s/cmdline", pid))
	if err != nil {
		return "", "", fmt.Errorf("Couldn't read arguments for pid %d: %s", pid, err)
	}

	// Items in cmdline files are separated with 0x00 byte
	cmdline := strings.Join(strings.Split(string(raw), "\x00"), " ")
	return exe, cmdline, nil
}

func (fc procRunningCheck) Run(p *Params) error {
	files, err := ioutil.ReadDir("/proc")
	if err != nil {
		return err
	}

	for _, file := range files {
		if num.MatchString(file.Name()) {
			exe, args, err := fetchProcessExeAndArgs(file.Name())

			if err != nil {
				continue
			}

			failed := false
			if p.Path != "" && !strings.Contains(exe, p.Path) {
				failed = true
			}

			if p.Check != "" && !strings.Contains(args, p.Check) {
				failed = true
			}

			if !failed {
				log.Printf("Found requested process (%s)", file.Name())
				return nil
			}
		}
	}
	return errors.New("Didn't find requested process")
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
	mu          sync.RWMutex
)

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
	mu.RLock()
	defer mu.RUnlock()

	ch, ok = typeToCheck[typ]
	return
}
