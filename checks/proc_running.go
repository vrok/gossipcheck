package checks

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

// procRunningCheck implements a checker that checks if a process is running.
// Currently, it only works on systems with procfs.
// It can be configured in to ways (if both are specified, they are ANDed):
//   - by specifying (a part of) path of the executable file in "path"
//   - by specifying (a part of) command used to execute it in "check"
// The latter greps through actual commands, so e.g. if a symlink was used
// to run a process, that's what it will see. The former will see the
// real file.
type procRunningCheck struct{}

func (fc procRunningCheck) Type() CheckType { return CheckProcRunning }

var num = regexp.MustCompile("^\\d+$")

func fetchProcessExeAndArgs(pid string) (exe string, args string, err error) {
	exe, err = os.Readlink(fmt.Sprintf("/proc/%s/exe", pid))
	if err != nil {
		return "", "", fmt.Errorf("Couldn't read link for pid %s: %s", pid, err)
	}

	raw, err := ioutil.ReadFile(fmt.Sprintf("/proc/%s/cmdline", pid))
	if err != nil {
		return "", "", fmt.Errorf("Couldn't read arguments for pid %s: %s", pid, err)
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
