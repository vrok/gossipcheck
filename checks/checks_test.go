package checks

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
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

func TestParamsGroup(t *testing.T) {
	cases := []struct {
		params ParamsGroup
		errors int
	}{
		{
			ParamsGroup{
				&Params{Type: "check_empty", Check: "not_empty"}, // Fail
				&Params{Type: "check_empty"},                     // Succeed
				&Params{Type: "check_empty", Check: "not_empty"}, // Fail
			},
			2,
		},
		{
			ParamsGroup{
				&Params{Type: "check_empty"}, // Succeed
			},
			0,
		},
		{
			ParamsGroup{
				&Params{Type: "check_empty", Check: "not_empty"}, // Fail
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

func TestProcRunningCheck(t *testing.T) {
	ch := procRunningCheck{}

	sleepPath, err := exec.LookPath("sleep")
	if err != nil {
		t.Fatal("Couldn't find sleep", err)
	}

	cases := []struct {
		shArg string
		check Params
		found bool
	}{
		{
			"sleep 1000",
			Params{Path: sleepPath},
			true,
		},
		{
			"sleep 1001",
			Params{Path: "/bin/lets_hope_it_doesnt_exist"},
			false,
		},
		{
			"sleep 1002",
			Params{Check: "999"},
			false,
		},
		{
			"sleep 1003",
			Params{Check: "1003"},
			true,
		},
	}

	for _, c := range cases {
		tokens := strings.Split(c.shArg, " ")

		cmd := exec.Command(tokens[0], tokens[1:]...)
		err = cmd.Start()
		if err != nil {
			t.Fatal(err)
		}

		if c.found != (ch.Run(&c.check) == nil) {
			t.Fatal("Error detecting process")
		}
		cmd.Process.Kill()
	}
}
