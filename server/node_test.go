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

	msg := nodes[0].NewMessage(RunChecks, []*checks.Params{
		&checks.Params{Type: "check_empty"},
	})

	nodes[0].SendMsg(msg, nodes[0].Members()[:2])

	//nodes[0].Send([]byte("bla"))

	//time.Sleep(2 * time.Second)

	time.Sleep(5 * time.Second)
}

func TestSelectPeers(t *testing.T) {
	cases := []struct {
		members []*memberlist.Node
		excepts []string
		k       int
	}{
		{
			[]*memberlist.Node{
				&memberlist.Node{Name: "a"},
				&memberlist.Node{Name: "b"},
				&memberlist.Node{Name: "c"},
				&memberlist.Node{Name: "d"},
				&memberlist.Node{Name: "e"},
			},
			[]string{"a", "c"},
			2,
		},
		{
			[]*memberlist.Node{
				&memberlist.Node{Name: "a"},
				&memberlist.Node{Name: "b"},
			},
			[]string{},
			2,
		},
	}

	for _, c := range cases {
		r := selectPeers(c.k, c.members, c.excepts)

		if len(r) != c.k {
			t.Fatalf("Wrong number of members selected: %d", len(r))
		}

		for i := 0; i < len(r); i++ {
			for j := i + 1; j < len(r); j++ {
				if r[i].Name == r[j].Name {
					t.Fatal("The same member selected more than once")
				}
			}
		}

		for _, e := range c.excepts {
			for _, m := range r {
				if e == m.Name {
					t.Fatal("Excluded member selected")
				}
			}
		}
	}
}
