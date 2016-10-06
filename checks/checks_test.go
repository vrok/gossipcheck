package checks

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
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

func TestParamsGroup(t *testing.T) {
	cases := []struct {
		params ParamsGroup
		errors int
	}{
		{
			ParamsGroup{
				&Params{Name: "a", Type: "check_empty", Check: "not_empty"}, // Fail
				&Params{Name: "b", Type: "check_empty"},                     // Succeed
				&Params{Name: "c", Type: "check_empty", Check: "not_empty"}, // Fail
			},
			2,
		},
		{
			ParamsGroup{
				&Params{Name: "a", Type: "check_empty"}, // Succeed
			},
			0,
		},
		{
			ParamsGroup{
				&Params{Name: "a", Type: "check_empty", Check: "not_empty"}, // Fail
			},
			1,
		},
	}

	for _, c := range cases {
		errs := c.params.Run()
		if len(errs) != c.errors {
			t.Fatalf("Wrong number of errors: %d", len(errs))
		}
	}
}

func TestRunAction(t *testing.T) {
	sigusr1 := make(chan os.Signal, 1)
	signal.Notify(sigusr1, syscall.SIGUSR1)
	defer signal.Reset(syscall.SIGUSR1)

	cases := []struct {
		pg         ParamsGroup
		shouldExec bool
	}{
		{
			ParamsGroup{
				&Params{
					Type:   "check_empty",
					Check:  "not_empty",
					Action: fmt.Sprintf("kill -s USR1 %d", os.Getpid()),
				},
			},
			true,
		},
		{
			ParamsGroup{
				&Params{
					Type:   "check_empty",
					Action: fmt.Sprintf("kill -s USR1 %d", os.Getpid()),
				},
			},
			false,
		},
	}

	for _, c := range cases {
		c.pg.Run()

		received := false
		select {
		case <-sigusr1:
			// kill was run
			received = true
		case <-time.After(100 * time.Millisecond):
		}

		if received != c.shouldExec {
			t.Fatal("Action run failed")
		}
	}
}
