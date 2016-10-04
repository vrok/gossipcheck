package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/rpc"
	"os"
	"sort"

	"github.com/vrok/gossipcheck/checks"
	"github.com/vrok/gossipcheck/server"
)

var (
	serverAddr = flag.String("server", "127.0.0.1:5924", "The address where CLI client connects to")
	checksFile = flag.String("file", "", "JSON file with checks to run")
)

type sliceOfParams []*checks.Params

func (s sliceOfParams) Len() int           { return len(s) }
func (s sliceOfParams) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s sliceOfParams) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func loadChecks() ([]*checks.Params, error) {
	if *checksFile == "" {
		return nil, fmt.Errorf("Checks file not specified")
	}

	b, err := ioutil.ReadFile(*checksFile)
	if err != nil {
		return nil, err
	}

	var p map[string]checks.Params
	err = json.Unmarshal(b, &p)
	if err != nil {
		return nil, err
	}

	list := make([]*checks.Params, 0, len(p))

	for name, params := range p {
		params := params
		params.Name = name
		list = append(list, &params)
	}

	// Sort alphabetically by name.
	sort.Sort(sliceOfParams(list))

	return list, nil
}

func check(scope string, args []string) error {
	var err error
	var chArgs server.Args
	var chResult server.Result

	chArgs.Params, err = loadChecks()
	if err != nil {
		return err
	}

	client, err := rpc.DialHTTP("tcp", *serverAddr)
	if err != nil {
		return err
	}

	err = client.Call("CLIServer.Run"+scope+"Check", &chArgs, &chResult)

	if err != nil {
		return err
	}
	return nil
}

func usage(oldUsage func()) func() {
	return func() {
		oldUsage()

		fmt.Printf("Commands:\n")

		for _, cmd := range []struct {
			name, desc string
		}{
			{"check", "Run checks on the whole cluster."},
			{"local-check", "Run checks on just one server (useful for validation)."},
			{"list-members", "List members of the cluster known to the CLI server."},
		} {
			fmt.Printf("\t%s\n\t\t%s\n", cmd.name, cmd.desc)
		}
	}
}

func main() {
	flag.Parse()
	flag.Usage = usage(flag.Usage)

	var args = flag.Args()

	if len(args) == 0 {
		flag.Usage()
		return
	}

	var err error

	switch args[0] {
	case "check":
		err = check("Global", args[1:])
	case "local-check":
		err = check("Local", args[1:])
	case "list-members":
	case "help":
		flag.Usage()
	default:
		err = fmt.Errorf("Unknown command: %s\n", args[0])
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}
