package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/vrok/gossipcheck/server"
)

var (
	bind        = flag.String("bind", ":3505", "The address used for communication with other nodes")
	cliBind     = flag.String("cli-bind", "127.0.0.1:5924", "The address where CLI client connects to")
	noCLI       = flag.Bool("no-cli", false, "Don't start command line RPC server")
	peers       = flag.String("peers", "", "Comma-separated list of addresses of initial peers (empty for the first node)")
	gossipGroup = flag.Int("gossip-group", 5, "Number of nodes that this node will talk to in every iteration")
)

func main() {
	flag.Parse()

	var hosts []string
	if *peers != "" {
		hosts = strings.Split(*peers, ",")
	}

	n, err := server.NewNode(*bind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting: %s", err)
		os.Exit(1)
	}

	n.GossipNodes = *gossipGroup

	if !*noCLI {
		err = server.StartCLIServer(*cliBind, n)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting command line RPC server: %s", err)
			os.Exit(1)
		}
	}

	err = n.Join(hosts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error joining the cluster: %s", err)
		os.Exit(1)
	}

	log.Print("gossipcheckd has started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	n.Shutdown()
}
