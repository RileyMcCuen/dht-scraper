package dht

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	b "dht/bencode"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
)

type (
	Requester interface {
		Port() int
		Addr() net.Addr
	}
	UDPRequester struct {
		*net.UDPAddr
	}
	TCPRequester struct {
		*net.TCPAddr
	}
	Node struct {
		ID []byte
		net.IP
		port uint16
	}
	Message struct {
		Data      b.Dict
		Requester Requester
	}
	TorrentHash struct {
		Hash      b.String
		Requester Requester
	}
	Sender interface {
		Send(Message)
	}
	udpSender struct {
		q    chan Message
		conn *net.UDPConn
	}
	MetaLoader interface {
		Load(TorrentHash)
	}
	setMetaLoader struct {
		set map[string]b.String
	}
)

const (
	MaxPort            = 65536
	bitsInByte         = 8
	BytesInID          = 20
	halfBytesInID      = BytesInID / 2
	compressedNodeSize = 26
	ipEnd              = BytesInID + 4
)

var (
	zeroIP net.IP = net.IP{0, 0, 0, 0}
)

func RandID() ([]byte, error) {
	randBytes := make([]byte, BytesInID)
	n, err := rand.Read(randBytes)
	if err != nil {
		return nil, err
	}
	if n != BytesInID {
		return nil, errors.New("not enough random bytes were generated")
	}
	hash := sha1.New()
	if _, err = hash.Write(randBytes); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

func NeighborID(target, nid b.String) b.String {
	return append(append(make(b.String, 0), target[:halfBytesInID]...), nid[halfBytesInID:]...)
}

func (ur UDPRequester) Port() int      { return ur.UDPAddr.Port }
func (ur UDPRequester) Addr() net.Addr { return ur.UDPAddr }

func (tr TCPRequester) Port() int      { return tr.TCPAddr.Port }
func (tr TCPRequester) Addr() net.Addr { return tr.TCPAddr }

func ResolveNode(network, address string, port int) (Node, error) {
	ret := Node{}
	addr, err := net.ResolveUDPAddr(network, address+":"+fmt.Sprint(port))
	if err != nil {
		return ret, err
	}
	id, err := RandID()
	if err != nil {
		return ret, err
	}
	if port >= MaxPort {
		return ret, errors.New("cannot resolve node, port is invalid")
	}
	ret.port, ret.ID, ret.IP = uint16(port), id, addr.IP
	return ret, nil
}

func ParseNode(data []byte) Node {
	id, address, port := data[:BytesInID], data[BytesInID:ipEnd], data[ipEnd:]
	return Node{
		id,
		address,
		binary.BigEndian.Uint16(port),
	}
}

func (n Node) Valid(id b.String) bool {
	return !net.IP.Equal(n.IP, zeroIP) && !bytes.Equal(n.ID, id)
}

func (n Node) String() string {
	portBytes := []byte{0, 0}
	binary.BigEndian.PutUint16(portBytes, n.port)
	return fmt.Sprintf("%s%s%s", n.ID, []byte(n.IP), portBytes)
}

func (n Node) Port() int      { return int(n.port) }
func (n Node) Addr() net.Addr { return &net.UDPAddr{IP: n.IP, Port: int(n.port), Zone: ""} }

func ParseNodes(data []byte) ([]Node, error) {
	if len(data)%compressedNodeSize != 0 {
		return nil, errors.New("compact nodes string was invalid, wrong size")
	}
	nodes := make([]Node, 0, len(data)/compressedNodeSize)
	for i := 0; i < len(data)-compressedNodeSize; i += compressedNodeSize {
		nodes = append(nodes, ParseNode(data[i:i+compressedNodeSize]))
	}
	return nodes, nil
}

func NewUDPSender(queueSize int, conn *net.UDPConn) *udpSender {
	ret := &udpSender{
		make(chan Message, queueSize),
		conn,
	}
	go ret.send()
	return ret
}

func (s *udpSender) send() {
	for {
		m := <-s.q
		if _, err := s.conn.WriteTo(m.Data.Bytes(), m.Requester.Addr()); err != nil {
			log.Println("While writing to udpconn:", err)
		}
	}
}

func (s *udpSender) Send(m Message) { s.q <- m }

func NewDownloader() MetaLoader {
	return &setMetaLoader{make(map[string]b.String)}
}

func (d *setMetaLoader) Load(t TorrentHash) {
	raw := t.Hash.Raw()
	if _, ok := d.set[raw]; ok {
		// already in the set, not a new hash
		return
	}
	d.set[raw] = t.Hash
	log.Println("New hash added:", t.Hash.Bytes())
}
