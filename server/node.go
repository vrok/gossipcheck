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

// Node represents a running node in the gossipcheck cluster.
type Node struct {
	// Number of nodes that messages are send to directly from a node.
	// It has the same meaning as GossipNodes in the memberlist library,
	// but is used for checks. By default, GossipNodes is set to the
	// same value as in memberlist (can be set to something different,
	// but only before Node.Join is called).
	GossipNodes int
	// Every AdvertDelay, a node advertises to a number of random nodes
	// what messages it has. Nodes can then request missing ones.
	AdvertInterval time.Duration

	name    string
	port    int
	config  *memberlist.Config
	list    *memberlist.Memberlist
	history *History

	// Closing this chan shuts everything down.
	done chan struct{}
}

// NewNode creates a new cluster node that will listen on the given address.
func NewNode(bind string) (*Node, error) {
	_, ps, err := net.SplitHostPort(bind)
	if err != nil {
		return nil, errors.New("Invalid bind address: " + err.Error())
	}

	port, err := strconv.ParseInt(ps, 10, 64)
	if err != nil {
		return nil, errors.New("Invalid port: " + err.Error())
	}

	node := &Node{
		port:    int(port),
		history: NewHistory(1000000, 2000),
		done:    make(chan struct{}),
	}

	config := memberlist.DefaultLANConfig()
	// memberlist needs unique names to work properly
	config.Name += "_" + randStr(12)
	node.name = config.Name
	config.BindPort = int(port)
	config.AdvertisePort = int(port)
	config.Delegate = &delegate{n: node}

	node.config = config
	node.GossipNodes = config.GossipNodes
	node.AdvertInterval = 20 * time.Second

	return node, nil
}

// Join starts the node, it then tries to join the cluster by connecting to the given
// peer node addresses.
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

// selectPeers randomly select at most "count" number of random nodes from
// "members", avoiding those whose names are in "excepts".
// This is very similar to memberlist's method.
func selectPeers(count int, members []*memberlist.Node, excepts []string) []*memberlist.Node {
	l := len(members)
	var selected []*memberlist.Node

	exceptSet := make(map[string]struct{}, len(excepts))
	for _, e := range excepts {
		exceptSet[e] = struct{}{}
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

		_, ok := exceptSet[name]
		if ok {
			continue outer
		}

		exceptSet[name] = struct{}{}
		selected = append(selected, members[n])
	}
	return selected
}

// SendMsg sends a message to a list of peer nodes.
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

// Shutdown stop the node gracefully.
func (n *Node) Shutdown() {
	close(n.done)
	if err := n.list.Leave(1 * time.Second); err != nil {
		log.Print("Error leaving the cluster: " + err.Error())
	}
	if err := n.list.Shutdown(); err != nil {
		log.Print("Error shutting down: " + err.Error())
	}
}

// runAdvertiser starts the advertiser goroutine, which periodically advertises
// IDs of messages it remembers to a number of random nodes. This is in the same
// vein as message dissemination in the SWIM protocol (the main difference is
// that IDs of all messages are sent).
func (n *Node) runAdvertiser() {
	go func() {
		for {
			select {
			case <-time.After(n.AdvertInterval):
				msg := n.NewMessage(AdvertiseMsgs)
				msg.MessageIDs = n.history.MessageIDs()
				if len(msg.MessageIDs) == 0 {
					continue
				}

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

// findPeer returns a node with the given ID.
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

// Members returns a list of all cluster members known to the node.
func (n *Node) Members() []*memberlist.Node {
	// Memberlist.Members() is thread-safe.
	return n.list.Members()
}

// Implements memberlist.Delegate
type delegate struct {
	n *Node
}

// NotifyMsg is the only method in memberlist.Delegate we need. It is run
// when message from a peer node arrives.
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

// Stub functions to satisfy the memberlist.Delegate interface.
func (d *delegate) NodeMeta(limit int) []byte                  { return nil }
func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte { return nil }
func (d *delegate) LocalState(join bool) []byte                { return nil }
func (d *delegate) MergeRemoteState(buf []byte, join bool)     {}

// ProcessMsg handles arriving messages. It is used both internally (for incoming messages),
// and externally (e.g. the CLI server simply runs ProcessNode with a locally-created message).
func (n *Node) ProcessMsg(m *Message) error {
	if !m.IsOneOff() && n.history.Observe(m) {
		// Already processed a message with this ID.
		log.Printf("Received an old message %s", m.Type)
		return nil
	}

	switch m.Type {
	case RunChecks:
		return n.processRunChecks(m)
	case AdvertiseMsgs:
		return n.processAdvertiseMsgs(m)
	case RequestMsgs:
		return n.processRequestMsgs(m)
	default:
		return errors.New("Unknown message type")
	}
}

func (n *Node) processRunChecks(m *Message) error {
	log.Print("Received new checks to run")
	go m.Params.Run()
	peers := selectPeers(n.GossipNodes, n.Members(), []string{m.SrcNode, m.OrigNode, n.name})
	m.SrcNode = n.name
	return n.SendMsg(m, peers)
}

func (n *Node) processAdvertiseMsgs(m *Message) error {
	missing := n.history.MissingIDs(m.MessageIDs)
	if len(missing) > 0 {
		reqMsg := n.NewMessage(RequestMsgs)
		reqMsg.MessageIDs = missing

		peer := n.findPeer(m.OrigNode)
		if peer == nil {
			return errors.New("Requesting node disappeared")
		}
		return n.SendMsg(reqMsg, []*memberlist.Node{peer})
	}
	return nil
}

func (n *Node) processRequestMsgs(m *Message) error {
	msgs := n.history.GetMessages(m.MessageIDs)
	if len(msgs) == 0 {
		return nil
	}
	peer := n.findPeer(m.OrigNode)
	if peer == nil {
		return errors.New("Requesting node disappeared")
	}

	for _, m := range msgs {
		mCopy := *m            // Shallow copy, but it's fine
		mCopy.SrcNode = n.name // Actually, this should already be set
		err := n.SendMsg(&mCopy, []*memberlist.Node{peer})
		if err != nil {
			// Stop on the first error, if something's wrong with the network
			// then further tries will probably fail too.
			return err
		}
	}
	return nil
}
