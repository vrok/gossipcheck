package server

import (
	"errors"
	"log"
	"net/rpc"
	"testing"

	"github.com/vrok/gossipcheck/checks"
)

func TestCli(t *testing.T) {
	addr := "localhost:4857"
	StartCLIServer(addr, nil)
	client, err := rpc.DialHTTP("tcp", addr)
	if err != nil {
		log.Fatal("dialing:", err)
	}

	cases := []struct {
		params *checks.Params
		err    error
	}{
		{
			&checks.Params{Type: "check_empty"},
			nil,
		},
		{
			&checks.Params{Type: "check_empty", Check: "Something"},
			errors.New("Check is not empty"),
		},
	}

	for _, c := range cases {
		var args Args
		var result Result

		args.Params = checks.ParamsGroup{c.params}
		err = client.Call("CLIServer.RunLocalCheck", &args, &result)
		if err != c.err && err.Error() != c.err.Error() {
			t.Error(err)
		}
	}
}
