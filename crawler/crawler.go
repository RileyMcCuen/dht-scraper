package crawler

import (
	"dht"
	b "dht/bencode"
	"dht/bittorrent"
	"errors"
	"log"
	"net"
	"reflect"
	"time"
)

type (
	Crawler interface {
		HandleResponse(dht.Requester, b.Dict) error
		HandleQuery(dht.Requester, b.Dict) error
		Start([]dht.Node) error
	}
	getMessage struct {
		Args struct {
			ID   b.String
			Hash string
		}
		Query         string
		TransactionID b.String
		MessageType   string
	}
	announceMessage struct {
		Args struct {
			ID          b.String
			ImpliedPort uint64
			Hash        b.String
			Port        uint64
			Token       b.String
		}
		Query         string
		TransactionID b.String
		MessageType   string
	}
	crawler struct {
		port       uint16
		sender     dht.Sender
		downloader dht.MetaLoader
		clientID   b.String
		// TODO: better data structure for nodes
		nodes []dht.Node
	}
)

const (
	tokenLength = 2
)

func repeat(d time.Duration, f func()) {
	t := time.NewTicker(time.Second)
	for {
		<-t.C
		f()
	}
}

func New(port uint16, sender dht.Sender, downloader dht.MetaLoader, clientID b.String) Crawler {
	return &crawler{port, sender, downloader, clientID, make([]dht.Node, 0)}
}

func (c *crawler) HandleResponse(req dht.Requester, d b.Dict) error {
	resp, err := d.GetDict(dht.ResponseKey)
	if err != nil {
		return err
	}
	nodesStr, err := resp.GetString(dht.ResponseNodes)
	if err != nil {
		return err
	}
	nodes, err := dht.ParseNodes(nodesStr)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.Valid(c.clientID) {
			c.nodes = append(c.nodes, node)
		}
	}
	return nil
}

func (c *crawler) handleGet(req dht.Requester, d b.Dict) error {
	r := &getMessage{}
	if err := d.Unmarshal(reflect.ValueOf(r)); err != nil {
		return err
	}
	hash, id := r.Args.Hash, r.Args.ID
	if len(hash) != dht.BytesInID || len(id) != dht.BytesInID {
		return errors.New("hash or id have incorrect length")
	}
	token := hash[:tokenLength]
	c.sender.Send(dht.Message{
		Data: b.D(
			b.P(dht.ResponseKey, b.D(
				b.P(dht.IDKey, dht.NeighborID(b.S(hash), id)),
				b.P(dht.ResponseNodes, dht.Empty),
				b.P(dht.TokenKey, b.S(token)),
			)),
			b.P(dht.TransactionID, r.TransactionID),
			b.P(dht.MessageType, dht.ResponseType),
		),
		Requester: req,
	})
	return nil
}

func (c *crawler) handleAnnounce(req dht.Requester, d b.Dict) error {
	r := &announceMessage{}
	if err := d.Unmarshal(reflect.ValueOf(r)); err != nil {
		return err
	}
	port := r.Args.Port
	if r.Args.ImpliedPort != 0 {
		port = uint64(req.Port())
	}
	hash := r.Args.Hash
	if !hash[:tokenLength].Equal(r.Args.Token) {
		return errors.New("invalid token in announce request")
	} else if port >= dht.MaxPort {
		return errors.New("port is invalid")
	}
	c.sender.Send(dht.Message{
		Data: b.D(
			b.P(dht.ResponseKey, b.D(
				b.P(dht.IDKey, dht.NeighborID(hash, r.Args.ID)),
			)),
			b.P(dht.TransactionID, r.TransactionID),
			b.P(dht.MessageType, dht.ResponseKey),
		),
		Requester: req,
	})
	c.downloader.Load(dht.TorrentHash{
		Hash:      r.Args.Hash,
		Requester: req,
	})
	return nil
}

func (c *crawler) HandleQuery(req dht.Requester, d b.Dict) error {
	query, err := d.GetString(dht.QueryKey)
	if err != nil {
		return err
	}
	switch {
	case dht.QueryGet.Equal(query):
		return c.handleGet(req, d)
	case dht.QueryAnnounce.Equal(query):
		return c.handleAnnounce(req, d)
	}
	return errors.New("cannot handle query type: " + query.Raw())
}

func (c *crawler) sendFindRequest(node dht.Node) {
	token, err := dht.RandID()
	if err != nil {
		log.Println("Could not get new token for find request", err)
		return
	}
	target, err := dht.RandID()
	if err != nil {
		log.Println("Could not get new target ID for find request", err)
		return
	}
	c.sender.Send(dht.Message{
		Data: b.D(
			b.P(dht.QueryArgs, b.D(
				b.P(dht.IDKey, c.clientID),
				b.P(dht.TargetKey, b.String(target)),
			)),
			b.P(dht.QueryKey, dht.QueryFind),
			b.P(dht.TransactionID, b.String(token[:tokenLength])),
			b.P(dht.MessageType, dht.QueryType),
		),
		Requester: node,
	})
}

func (c *crawler) makeNeighbors(bootstrapNodes []dht.Node) {
	repeat(time.Second, func() {
		nodes := c.nodes
		c.nodes = append([]dht.Node{}, bootstrapNodes...)
		for _, node := range nodes {
			if !node.Valid(c.clientID) {
				log.Println("Skipping make neighbor for:", node)
			} else {
				c.sendFindRequest(node)
			}
		}
	})
}

func (c *crawler) Start(bootstrapNodes []dht.Node) error {
	go c.makeNeighbors(bootstrapNodes)
	return nil
}

func (c *crawler) getMetaData(hash []byte, r dht.Requester) error {
	// connet over tcp
	// bittorrent handshake
	// accept extended handshake with port information from peer
	// ping port on peer
	conn, err := net.Dial("tcp4", r.Addr().String())
	if err != nil {
		return err
	}
	defer conn.Close()
	w := bittorrent.NewWire(conn, 2<<18)
	if err := w.Send(bittorrent.Handshake{Extension: bittorrent.DHT, Hash: hash, PeerID: c.clientID}); err != nil {
		return err
	}
	h, err := w.ReceiveHandshake()
	if err != nil {
		return err
	}
	if h.Extension != bittorrent.DHT {
		return errors.New("requester does not support DHT")
	}
	eh, err := w.ReadMessage()
	if err != nil {
		return err
	}
	extH, ok := eh.(bittorrent.ExtendedHandshake)
	if !ok {
		return errors.New("did not receive extended handshake response")
	}
	_ = extH.Dict
	return nil
}
