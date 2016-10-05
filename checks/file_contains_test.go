package checks

import (
	"io/ioutil"
	"os"
	"testing"
)

func createTempFile(content string) (name string, err error) {
	f, err := ioutil.TempFile("", "checks")
	if err != nil {
		return "", err
	}

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

	for i, c := range cases {
		f, err := createTempFile(c.content)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f)

		err = ch.Run(&Params{Path: f, Check: c.sep})
		if (err == nil) != c.found {
			t.Fatalf("Wrong result in case %d, %s", i, err)
		}
	}
}
