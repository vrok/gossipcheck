package checks

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
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

	pg := ParamsGroup{
		&Params{
			Type:   "check_empty",
			Check:  "not_empty",
			Action: fmt.Sprintf("kill -s USR1 %d", os.Getpid()),
		},
	}

	pg.Run()

	select {
	case <-sigusr1:
		// kill was run
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out")
	}
}

func createTempFile(content string) (name string, err error) {
	f, err := ioutil.TempFile("", "checks")
	if err != nil {
		return "", err
	}
	//defer os.Remove(f.Name())

	err = ioutil.WriteFile(f.Name(), []byte(content), 0600)
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}

	return f.Name(), nil
}

func TestFileContains(t *testing.T) {
	// This doesn't check searching per se because we're currently relying
	// heavily on strings.Contains. It rather tests whether we glue batches
	// passed to strings.Contains correctly.
	// These tests will probably need updating if we ever replace the search
	// algorithm.
	cases := []struct {
		content, sep string
		found        bool
	}{
		{
			"aaaaaa", "aa", true,
		},
		{
			"abcdefghij", "fg", true,
		},
		{
			"abcdefghij", "gf", false,
		},
		{
			"", "de", false,
		},
		{
			"a", "de", false,
		},
		{
			"abcde", "de", true,
		},
		{
			"abcdef", "ef", true,
		},
		{
			"abcdefg", "fg", true,
		},
		{
			"aaaaaaaaaaaaaaaaaa", "a", true,
		},
	}

	ch := fileContainsCheck{2}

	for _, c := range cases {
		f, err := createTempFile(c.content)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f)

		err = ch.Run(&Params{Path: f, Check: c.sep})
		if (err == nil) != c.found {
			t.Fatal("Wrong result")
		}
	}
}

func TestFileExistsCheck(t *testing.T) {
	f, err := ioutil.TempFile("", "checks")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	ch := fileExistsCheck{}

	if ch.Run(&Params{Path: f.Name()}) != nil {
		t.Fatal("File exists")
	}

	// Let's hope this file doesn't exist!
	if ch.Run(&Params{Path: "sdfsifjsifjwufje"}) == nil {
		t.Fatal("File doesn't exist")
	}
}
