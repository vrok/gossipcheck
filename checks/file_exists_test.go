package checks

import (
	"io/ioutil"
	"os"
	"testing"
)

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
