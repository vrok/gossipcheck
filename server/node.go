package server

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/memberlist"
)

type Node struct {
	name    string
	port    int
	config  *memberlist.Config
	list    *memberlist.Memberlist
	history *History
	// Number of nodes that messages are send directly from a node.
	// Has the same meaning as GossipNodes in the memberlist library,
	// but is used for checks. By default, GossipNodes is set to the
	// same value as in memberlist (can be set to something different,
	// but only before Node.Join is called).
	GossipNodes int
	// Every AdvertDelay, a node advertises to a number of random nodes
	// what messages it has. Nodes can then request missing ones.
	AdvertInterval time.Duration

	// Closing this chan shuts everything down.
	done chan struct{}
}

// Implements memberlist.Delegate
type delegate struct {
	n *Node
}

func (d *delegate) NodeMeta(limit int) []byte { return nil }
func (d *delegate) NotifyMsg(msg []byte) {
	buf := bytes.NewBuffer(msg)
	dec := gob.NewDecoder(buf)

	var m Message
	err := dec.Decode(&m)
	if err != nil {
		log.Println("Received a malformed message")
		return
	}

	d.n.ProcessMsg(&m)
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
		history: NewHistory(1000000, 2000),
		done:    make(chan struct{}),
	}

	config := memberlist.DefaultLocalConfig()
	//config := memberlist.DefaultLANConfig()
	config.Name += "_" + randStr(8) // memberlist needs unique names to work properly
	node.name = config.Name
	config.BindPort = int(port)
	config.AdvertisePort = int(port)
	config.Delegate = &delegate{n: node}

	node.config = config
	node.GossipNodes = config.GossipNodes
	//node.GossipNodes = 1
	// Don't use exactly the same value as GossipInterval, lest the network usage
	// spikes overlap.
	//node.AdvertInterval = config.GossipInterval * 3 / 2
	node.AdvertInterval = 20 * time.Second

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
	n.runAdvertiser()
	return nil
}

// NewMessage creates a new message that originates in the local node.
func (n *Node) NewMessage(typ MsgType) *Message {
	return &Message{
		Type:     typ,
		ID:       randStr(16),
		OrigNode: n.name,
		SrcNode:  n.name,
	}
}

func selectPeers(count int, members []*memberlist.Node, excepts []string) []*memberlist.Node {
	l := len(members)
	var selected []*memberlist.Node

	maxCount := len(members) - len(excepts)
	if count > maxCount {
		count = maxCount
	}

outer:
	// l*5 is a pretty exhaustive search, memberlist does sth similar, for small
	// sizes it's useful and for large ones it's not a problem (because
	// count << len(members)).
	// TODO(vrok): It can still sometimes return too few items, it's possible
	// to do it deterministically with O(k log k) (or maybe better) and not
	// necessarily O(n) (k - number of peers, n - cluster size).
	for i := 0; i < l*5 && len(selected) < count; i++ {
		n := rand.Intn(l)

		name := members[n].Name

		for _, e := range excepts {
			if e == name {
				continue outer
			}
		}

		excepts = append(excepts, name)
		selected = append(selected, members[n])
	}
	return selected
}

func (n *Node) SendMsg(m *Message, members []*memberlist.Node) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	err := enc.Encode(m)
	if err != nil {
		return err
	}

	data := buf.Bytes()

	// TODO: Check if SendToTCP is thread safe, if so then send it in parallel
	errCount := 0
	for _, node := range members {
		err := n.list.SendToTCP(node, data)
		if err != nil {
			log.Print("Error sending message: " + err.Error())
			errCount++
		}
	}

	if errCount == len(members) {
		// Return error only if the message wasn't sent even once.
		return errors.New("Sending message failed")
	}

	return nil
}

func (n *Node) Shutdown() {
	close(n.done)
	if err := n.list.Shutdown(); err != nil {
		log.Print("Error shutting down: " + err.Error())
	}
}

func (n *Node) runAdvertiser() {
	go func() {
		for {
			select {
			case <-time.After(n.AdvertInterval):
				msg := n.NewMessage(AdvertiseMsgs)
				msg.MessageIDs = n.history.MessageIDs()

				peers := selectPeers(n.GossipNodes, n.Members(), []string{n.name})
				err := n.SendMsg(msg, peers)
				if err != nil {
					log.Print("Error advertising messages: " + err.Error())
				}
			case <-n.done:
				return
			}
		}
	}()
}

func (n *Node) findPeer(id string) *memberlist.Node {
	// TODO(vrok): Number of nodes can be big, this linear search is lame. It would be easy
	// to wrap it in a cache once this pops up during profiling.
	for _, peer := range n.Members() {
		if peer.Name == id {
			return peer
		}
	}
	return nil
}

func (n *Node) ProcessMsg(m *Message) error {
	if n.history.Observe(m) {
		// Already processed a message with this ID.
		log.Print("Received an old message")
		return nil
	}

	switch m.Type {
	case RunChecks:
		log.Print("Received new checks to run")
		go m.Params.Run()
		peers := selectPeers(n.GossipNodes, n.Members(), []string{m.SrcNode, m.OrigNode, n.name})
		m.SrcNode = n.name
		return n.SendMsg(m, peers)
	case AdvertiseMsgs:
		missing := n.history.MissingIDs(m.MessageIDs)
		if len(missing) > 0 {
			reqMsg := n.NewMessage(ReqestMsgs)
			reqMsg.MessageIDs = missing

			peer := n.findPeer(m.OrigNode)
			if peer == nil {
				return errors.New("Requesting node disappeared")
			}
			return n.SendMsg(reqMsg, []*memberlist.Node{peer})
		}
	case ReqestMsgs:
		msgs := n.history.GetMessages(m.MessageIDs)
		if len(msgs) == 0 {
			return nil
		}
		peer := n.findPeer(m.OrigNode)
		if peer == nil {
			return errors.New("Requesting node disappeared")
		}

		for _, m := range msgs {
			m.SrcNode = n.name // Actually, this should already be set
			err := n.SendMsg(m, []*memberlist.Node{peer})
			if err != nil {
				// Stop on the first error, if something's wrong with the network
				// then further tries will probably fail too.
				return err
			}
		}
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
