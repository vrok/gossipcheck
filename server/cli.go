package server

import (
	"fmt"
	"net"
	"net/http"
	"net/rpc"

	"github.com/vrok/gossipcheck/checks"
)

// CLIServer represents a running command line RPC server that is attached
// to a gossip protocol node. Command line client can connect to it and
// send checks through it to the cluster.
type CLIServer struct {
	// Local node
	node *Node
}

// Args represents arguments for Go's RPC.
type Args struct {
	Params checks.ParamsGroup
}

// Result represents results for Go's RPC.
// Right now we only return errors so it's empty.
type Result struct{}

// RunGlobalCheck is an RPC-exposed function that sends a check to the
// attached node, which is then spread to the whole cluster.
func (s *CLIServer) RunGlobalCheck(args *Args, r *Result) error {
	msg := s.node.NewMessage(RunChecks)
	msg.Params = args.Params
	return s.node.ProcessMsg(msg)
}

// RunLocalCheck just runs the check locally. It is useful for testing
// check before running them on the whole cluster.
func (s *CLIServer) RunLocalCheck(args *Args, r *Result) error {
	for _, p := range args.Params {
		check, ok := checks.GetCheck(p.Type)
		if !ok {
			return fmt.Errorf("Check doesn't exist: %s", p.Type)
		}
		err := check.Run(p)
		if err != nil {
			return err
		}
	}
	return nil
}

// StartCLIServer starts a new RPC command line interface server
// that is attached to a Node instance.
func StartCLIServer(bind string, node *Node) error {
	cliServ := &CLIServer{node: node}
	err := rpc.Register(cliServ)
	if err != nil {
		return err
	}
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", bind)
	if err != nil {
		return err
	}
	go http.Serve(l, nil)
	return nil
}
