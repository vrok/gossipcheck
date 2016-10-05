package checks

import "os"

type fileExistsCheck struct{}

func (fe fileExistsCheck) Type() CheckType { return CheckFileExists }

func (fe fileExistsCheck) Run(p *Params) error {
	if _, err := os.Stat(p.Path); os.IsNotExist(err) {
		return err
	}
	return nil
}
