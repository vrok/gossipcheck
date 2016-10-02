package server

import (
	"errors"
	"fmt"
	"gossipcheck/checks"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"

	"github.com/hashicorp/memberlist"
)

type Node struct {
	name    string
	port    int
	config  *memberlist.Config
	list    *memberlist.Memberlist
	history *History
}

// Implements memberlist.Delegate
type delegate struct {
	n *Node
}

func (d *delegate) NodeMeta(limit int) []byte { return nil }
func (d *delegate) NotifyMsg(msg []byte) {
	fmt.Printf("ZZZ Received msg %s\n", string(msg))
}
func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte { return nil }
func (d *delegate) LocalState(join bool) []byte                { return nil }
func (d *delegate) MergeRemoteState(buf []byte, join bool)     {}

const lettersCnt = 'z' - 'a'

func randStr(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + rand.Int()%lettersCnt)
	}
	return string(b)
}

func NewNode(bind string) (*Node, error) {
	_, portS, err := net.SplitHostPort(bind)
	if err != nil {
		return nil, err
	}

	port, err := strconv.ParseInt(portS, 10, 64)
	if err != nil {
		return nil, err
	}

	node := &Node{
		port:    int(port),
		history: NewHistory(1000000),
	}

	config := memberlist.DefaultLocalConfig()
	//config := memberlist.DefaultLANConfig()
	config.Name += "_" + randStr(8) // memberlist needs unique names to work properly
	node.name = config.Name
	config.BindPort = int(port)
	config.AdvertisePort = int(port)
	config.Delegate = &delegate{n: node}

	node.config = config

	fmt.Printf("ZZZ Delegate %#v\n", config.Delegate)
	return node, nil
}

func (n *Node) Join(peers []string) error {
	var err error

	for i := range peers {
		// No port of a peer specified, use the same as the local node
		if strings.IndexByte(peers[i], ':') == -1 {
			peers[i] = peers[i] + ":" + string(n.port)
		}
	}

	n.list, err = memberlist.Create(n.config)
	if err != nil {
		return fmt.Errorf("Failed to create memberlist: %s", err)
	}

	cnt, err := n.list.Join(peers)
	if err != nil {
		return fmt.Errorf("Failed to join cluster: %s", err)
	}

	log.Printf("Node %s started, %d peers responded", n.config.Name, cnt)
	return nil
}

// NewMessage creates a new message that originates in the local node.
func (n *Node) NewMessage(typ MsgType, params []*checks.Params) *Message {
	return &Message{
		Type:     typ,
		ID:       randStr(16),
		OrigNode: n.name,
		SrcNode:  n.name,
		Params:   params,
	}
}

func (n *Node) ProcessMsg(m *Message) error {
	if n.history.Observe(m.ID) {
		// Already processed a message with this ID.
		return nil
	}

	switch m.Type {
	case RunChecks:
		panic("todo")
	case InstallChecks:
		panic("todo")
	case DeleteChecks:
		panic("todo")
	case CheckFailed:
		panic("todo")
	default:
		return errors.New("Unknown message type")
	}
	return nil
}

func (n *Node) Members() []*memberlist.Node {
	// Memberlist.Members() is thread-safe.
	return n.list.Members()
}

func (n *Node) Send(b []byte) {
	for _, peer := range n.Members() {
		n.list.SendToTCP(peer, b)
	}
}
