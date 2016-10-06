package checks

import (
	"errors"
	"io"
	"os"
	"strings"
)

// fileContainsCheck implements a checker that checks if a given text is in a file.
// It can handle very big files (e.g. logs).
type fileContainsCheck struct {
	// batchMult controls size of the batch that is loaded by fileContainsCheck.
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
