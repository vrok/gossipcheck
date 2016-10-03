package server

import (
	"errors"
	"fmt"
	"gossipcheck/checks"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
)

// Fake check that fails the first failsMax times and then always succeeds.
type checkFake struct {
	wg *sync.WaitGroup

	fails, failsMax int
	mu              sync.Mutex
}

func (fe *checkFake) Type() checks.CheckType { return "fake" }

func (fe *checkFake) Run(p *checks.Params) error {
	log.Print("Fake check running")

	fe.wg.Done()

	fe.mu.Lock()
	defer fe.mu.Unlock()

	fe.fails++
	if fe.fails <= fe.failsMax {
		return errors.New("Fake fail")
	}
	return nil
}

func TestNode(t *testing.T) {
	testProtocol(t, 5, 4)
	testProtocol(t, 20, 4)
	testProtocol(t, 50, 4)
	testProtocol(t, 50, 2)
	//testProtocol(t, 200, 5) // this can drain all file descriptors
	//testProtocol(t, 20, 1) // this will converge slowly
}

func testProtocol(t *testing.T, nodeCount, gossipGroup int) {
	var nodes []*Node
	var addrs []string

	probe := func() {
		for _, n := range nodes {
			fmt.Printf("%d ", len(n.Members()))
		}
		fmt.Println()
	}

	eventCh := make(chan memberlist.NodeEvent, 3*nodeCount*nodeCount)
	events := memberlist.ChannelEventDelegate{Ch: eventCh}

	wg := &sync.WaitGroup{}
	wg.Add(nodeCount)

	chk := &checkFake{wg: wg, fails: 1}
	checks.AddCheck(chk)

	for i := 0; i < nodeCount; i++ {
		addr := fmt.Sprintf("127.0.0.1:%d", 3400+i)
		fmt.Println("Starting " + addr)

		// Let it connect to all nodes so that we don't have to wait for SWIM to converge.
		n, err := NewNode(addr)
		if err != nil {
			t.Fatal(err)
		}

		n.config.Events = &events
		//n.config.GossipNodes = gossipGroup
		n.GossipNodes = gossipGroup

		//var jn []string
		//if len(addrs) > 0 {
		//	jn = append(jn, addrs[i-1])
		//}
		//err = n.Join(jn)

		// Join to all nodes - we don't want to test memberlist, so make it converge
		// as soon as possible.
		err = n.Join(addrs)
		if err != nil {
			t.Fatal(err)
		}
		addrs = append(addrs, addr)
		nodes = append(nodes, n)
	}

	// Wait for the protocol to converge.

	timeout := time.After(100 * time.Second)
	counter := 0
loop:
	for {
		select {
		case event := <-eventCh:
			if event.Event == memberlist.NodeJoin {
				counter++
				// Everyone knows about everytone
				if counter == nodeCount*nodeCount {
					break loop
				}
			}
			probe()
		case <-timeout:
			t.Fatal("Didn't converge in time")
		}
	}

	fmt.Println("Memberlist converged")

	msg := nodes[0].NewMessage(RunChecks)
	msg.Params = checks.ParamsGroup{
		&checks.Params{Type: "fake"},
	}

	// The first node sends a message.
	nodes[0].ProcessMsg(msg)

	fmt.Println("Waiting...")

	done := make(chan struct{})

	go func() {
		// Wait until every node processes the message.
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Every node got the message.
	case <-time.After(100 * time.Second):
		t.Fatal("Test timed out, not every node processed the message.")
	}

	for _, n := range nodes {
		n.Shutdown()
	}
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
