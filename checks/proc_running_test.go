// +build linux

package checks

import (
	"os/exec"
	"strings"
	"testing"
)

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
