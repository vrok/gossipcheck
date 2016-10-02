package server

import (
	"fmt"
	"gossipcheck/checks"
	"log"
	"net"
	"net/http"
	"net/rpc"
)

type CLIServer struct {
	// Local node
	node *Node
}

type Args struct {
	Params []*checks.Params
}

type Result struct{}

func (s *CLIServer) RunGlobalCheck(args *Args, r *Result) error {
	fmt.Printf("ZZZ Run global check %#v\n", *args)

	msg := s.node.NewMessage(RunChecks, args.Params)
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

func StartCLIServer(bind string) {
	cliServ := new(CLIServer)
	rpc.Register(cliServ)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", bind)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
	//http.Serve(l, nil)
}
