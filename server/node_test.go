package server

import (
	"fmt"
	"gossipcheck/checks"
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
)

type checkFake struct{ ch chan struct{} }

func (fe checkFake) Type() string { return "fake" }

func (fe checkFake) Run(p *checks.Params) error {
	fe.ch <- struct{}{}
	return nil
}

// Fake memberlist.EventDelegate
type eventDelegFake struct{ ch chan struct{} }

func newEventDelegFake() *eventDelegFake {
	return &eventDelegFake{make(chan struct{}, 50)}
}

func (d eventDelegFake) NotifyJoin(*Node)   { d.ch <- struct{}{} }
func (d eventDelegFake) NotifyLeave(*Node)  {}
func (d eventDelegFake) NotifyUpdate(*Node) {}

func TestNode(t *testing.T) {
	n := 5

	var nodes []*Node
	var addrs []string

	eventCh := make(chan memberlist.NodeEvent, 50)
	events := memberlist.ChannelEventDelegate{Ch: eventCh}

	for i := 0; i < n; i++ {
		addr := fmt.Sprintf("127.0.0.1:%d", 3400+i)
		fmt.Println("Starting " + addr)
		//ns := []string{}
		//if len(addrs) > 0 {
		//	ns = addrs[len(addrs)-1 : len(addrs)]
		//}
		//n, err := NewNode(addr, ns)

		// Let it connect to all nodes so that we don't have to wait for SWIM to converge.
		n, err := NewNode(addr)
		if err != nil {
			t.Fatal(err)
		}

		n.config.Events = &events

		err = n.Join(addrs)
		if err != nil {
			t.Fatal(err)
		}
		addrs = append(addrs, addr)
		nodes = append(nodes, n)
	}

	// Wait for the protocol to converge.

	timeout := time.After(2 * time.Second)
	counter := 0
loop:
	for {
		select {
		case event := <-eventCh:
			if event.Event == memberlist.NodeJoin {
				counter++
				// Everyone knows about everytone
				if counter == n*n {
					break loop
				}
			}
		case <-timeout:
			t.Fatal("Didn't converge in time")
		}
	}

	for i, n := range nodes {
		fmt.Printf("ZZZ node %d has %d peers\n", i, len(n.Members()))
	}

	time.Sleep(1 * time.Second)

	fmt.Println("")
	for i, n := range nodes {
		fmt.Printf("ZZZ node %d has %d peers\n", i, len(n.Members()))
	}

	nodes[0].Send([]byte("bla"))
	nodes[0].Send([]byte("bla"))

	//time.Sleep(2 * time.Second)

	time.Sleep(5 * time.Second)
}
