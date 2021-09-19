package dht

import (
	b "dht/bencode"
	"errors"
	"io"
	"log"
)

type (
	Handler        func(Requester, b.Dict) error
	MessageHandler interface {
		RegisterHandler(messageType b.String, f Handler) error
		Handle(Requester, io.Reader) error
	}
	messageHandler struct {
		handlers map[byte]Handler
	}
)

// Common keys for messages
var (
	// TransactionID key
	TransactionID = b.S("t")
	// MessageType key
	MessageType = b.S("y")
	// MessageTypes
	QueryType    = b.S("q")
	ResponseType = b.S("r")
	ErrorType    = b.S("e")
	// Query specific keys
	QueryKey  = QueryType
	QueryArgs = b.S("a")
	// Queries
	QueryPing     = b.S("ping")
	QueryFind     = b.S("find_node")
	QueryGet      = b.S("get_peers")
	QueryAnnounce = b.S("announce_peer")
	// Response specific keys
	ResponseKey   = ResponseType
	ResponseNodes = b.S("nodes")
	// Error specific keys
	ErrorKey = ErrorType
	// Other common keys
	IDKey     = b.S("id")
	HashKey   = b.S("info_hash")
	TokenKey  = b.S("token")
	TargetKey = b.S("target")
	// Other common values
	Empty = b.S("")
)

func Noop(b.Dict) error { return nil }
func LogOp(b b.Dict) error {
	log.Println("Pretty Message:\n", b.Pretty("", "    "))
	return nil
}

func New() MessageHandler {
	return &messageHandler{make(map[byte]Handler)}
}

func (mh *messageHandler) RegisterHandler(messageType b.String, f Handler) error {
	if messageType.Len() > 1 {
		return errors.New("MessageType field of message had more than one byte")
	} else {
		mh.handlers[messageType[0]] = f
	}
	return nil
}

func (mh *messageHandler) Handle(req Requester, r io.Reader) error {
	msg, err := b.Decode(r)
	if err != nil {
		return err
	}
	d, ok := msg.(b.Dict)
	if !ok {
		return errors.New("message was not a dict but should have been")
	}
	log.Println("handling message", req.Addr(), d.Keys())
	mt, err := d.GetString(MessageType)
	if err != nil {
		return err
	}
	if mt.Len() > 1 {
		return errors.New("MessageType field of message had more than one byte")
	}
	if handler := mh.handlers[mt[0]]; handler != nil {
		return handler(req, d)
	}
	return errors.New("no such handler for MessageType")
}
