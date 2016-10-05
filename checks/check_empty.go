package checks

import "errors"

// Useful for testing. Always succeeds when check is empty, fails otherwise.
type checkEmpty struct{}

func (fe checkEmpty) Type() CheckType { return "check_empty" }

func (fe checkEmpty) Run(p *Params) error {
	if p.Check != "" {
		return errors.New("Check is not empty")
	}
	return nil
}
