package main

import "flag"

var (
	bind    = flag.String("bind", ":3505", "The address used for communication with other nodes")
	cliBind = flag.String("cli-bind", "127.0.0.1:5924", "The address where CLI client connects to")
)

func main() {
}
