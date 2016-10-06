package checks

import "os"

// fileExistsCheck implements a checker that checks if a file exists.
type fileExistsCheck struct{}

func (fe fileExistsCheck) Type() CheckType { return CheckFileExists }

func (fe fileExistsCheck) Run(p *Params) error {
	if _, err := os.Stat(p.Path); os.IsNotExist(err) {
		return err
	}
	return nil
}
