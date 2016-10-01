package checks

import (
	"bytes"
	"encoding/gob"
	"errors"
	"testing"
)

func TestCustomGob(t *testing.T) {
	cases := []struct {
		ch  CheckType
		err error // Expected error
	}{
		{
			CheckFileContains,
			nil,
		},
		{
			CheckFileExists,
			nil,
		},
		{
			CheckProcRunning,
			nil,
		},
		{
			CheckType("something"),
			errors.New("Unexpected check"),
		},
	}

	for _, c := range cases {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		dec := gob.NewDecoder(&buf)

		err := enc.Encode(&c.ch)

		if err != nil {
			if err.Error() == c.err.Error() {
				// that's expected by the test case
				continue
			}
			t.Errorf("Unexpected error: %s", err)
		}

		var cht CheckType
		err = dec.Decode(&cht)
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		if cht != c.ch {
			t.Errorf("Value mutated after encode & decode")
		}
	}
}
