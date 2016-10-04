package server

import (
	"fmt"
	"gossipcheck/checks"
	"net"
	"net/http"
	"net/rpc"
)

type CLIServer struct {
	// Local node
	node *Node
}

type Args struct {
	Params checks.ParamsGroup
}

type Result struct{}

func (s *CLIServer) RunGlobalCheck(args *Args, r *Result) error {
	msg := s.node.NewMessage(RunChecks)
	msg.Params = args.Params
	return s.node.ProcessMsg(msg)
}

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
