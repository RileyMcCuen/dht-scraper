package bittorrent

import (
	"bytes"
	bf "dht/bitfield"
	"encoding/binary"
	"errors"
	"io"
)

type (
	streamer struct {
		io.ReadWriter
		buf []byte
		max int
	}
	extension byte
	Wire      struct {
		*streamer
	}
	Message interface {
		Kind() byte
		Write(*streamer) error
	}
	// Messages with no data
	KeepAlive     struct{}
	Choke         struct{}
	Unchoke       struct{}
	Interested    struct{}
	NotInterested struct{}
	// Messages with data
	Have     struct{ Index uint }
	BitField struct {
		*bf.BitField
	}
	Request struct {
		Index, Begin, Length uint
	}
	Piece struct {
		Index, Begin uint
		Piece        []byte
	}
	Cancel struct {
		Index, Begin, Length uint
	}
	Port struct{ Port uint16 }
	// Special Handshake Message
	Handshake struct {
		Extension    extension
		Hash, PeerID []byte
	}
)

const (
	numReservedBits      = 20 * 8
	choke           byte = iota - 1
	unchoke
	interested
	notInterested
	have
	bitfield
	request
	piece
	cancel
	port
	extended  byte = 20
	keepAlive byte = iota + 117
	handshake
	Original extension = iota - 14
	DHT
)

var (
	btPreamble = []byte("BitTorrent protocol")
)

func (KeepAlive) Kind() byte     { return keepAlive }
func (Handshake) Kind() byte     { return handshake }
func (Choke) Kind() byte         { return choke }
func (Unchoke) Kind() byte       { return unchoke }
func (Interested) Kind() byte    { return interested }
func (NotInterested) Kind() byte { return notInterested }
func (Have) Kind() byte          { return have }
func (BitField) Kind() byte      { return bitfield }
func (Request) Kind() byte       { return request }
func (Piece) Kind() byte         { return piece }
func (Cancel) Kind() byte        { return cancel }
func (Port) Kind() byte          { return port }

func (KeepAlive) Write(w *streamer) error     { return w.WriteNumbers(uint(0)) }
func (Handshake) Write(w *streamer) error     { return w.WriteNumbers(uint(1), handshake) }
func (Choke) Write(w *streamer) error         { return w.WriteNumbers(uint(1), choke) }
func (Unchoke) Write(w *streamer) error       { return w.WriteNumbers(uint(1), unchoke) }
func (Interested) Write(w *streamer) error    { return w.WriteNumbers(uint(1), interested) }
func (NotInterested) Write(w *streamer) error { return w.WriteNumbers(uint(1), notInterested) }
func (h Have) Write(w *streamer) error        { return w.WriteNumbers(uint(5), have, h.Index) }
func (b BitField) Write(w *streamer) error {
	if err := w.WriteNumbers(uint(b.NumBytes()), bitfield); err != nil {
		return err
	}
	return w.Write(b.Bytes()...)
}
func (r Request) Write(w *streamer) error {
	return w.WriteNumbers(uint(13), request, r.Index, r.Begin, r.Length)
}
func (p Piece) Write(w *streamer) error {
	if err := w.WriteNumbers(uint(len(p.Piece)+9), piece, p.Index, p.Begin); err != nil {
		return err
	}
	return w.Write(p.Piece...)
}
func (c Cancel) Write(w *streamer) error {
	return w.WriteNumbers(uint(13), cancel, c.Index, c.Begin, c.Length)
}
func (p Port) Write(w *streamer) error { return w.WriteNumbers(uint(3), port, p.Port) }

func newStreamer(rw io.ReadWriter, maxSize int) *streamer {
	return &streamer{rw, make([]byte, 0), maxSize}
}
func (s *streamer) grow(size int) []byte {
	if len(s.buf) < size {
		s.buf = make([]byte, size)
	}
	return s.buf[:size]
}
func (s *streamer) read(size int) error {
	if err := s.ReadRaw(s.grow(size)); err != nil {
		return err
	}
	return nil
}
func (s *streamer) ReadByte() (byte, error) {
	if err := s.read(1); err != nil {
		return 0, err
	}
	return s.buf[0], nil
}
func (s *streamer) ReadBytes(size int) ([]byte, error) {
	if err := s.read(size); err != nil {
		return nil, err
	}
	return s.buf[:size], nil
}
func (s *streamer) ReadRaw(p []byte) error {
	n, err := s.ReadWriter.Read(p)
	if err != nil && err != io.EOF {
		return err
	}
	if n != len(p) {
		return errors.New("read operation did not read desired number of bytes")
	}
	return nil
}
func (s *streamer) ReadRaws(ps ...[]byte) error {
	for _, p := range ps {
		if err := s.ReadRaw(p); err != nil {
			return err
		}
	}
	return nil
}
func (s *streamer) ReadNumber(num interface{}) error {
	return binary.Read(s.ReadWriter, binary.BigEndian, num)
}
func (s *streamer) ReadNumbers(nums ...interface{}) error {
	for _, num := range nums {
		if err := s.ReadNumber(num); err != nil {
			return err
		}
	}
	return nil
}
func (s *streamer) Write(data ...byte) error {
	n, err := s.ReadWriter.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return errors.New("could not write all data to stream")
	}
	return nil
}
func (s *streamer) WriteNumber(num interface{}) error {
	return binary.Write(s.ReadWriter, binary.BigEndian, num)
}
func (s *streamer) WriteNumbers(nums ...interface{}) error {
	for _, num := range nums {
		if err := s.WriteNumber(num); err != nil {
			return err
		}
	}
	return nil
}

func NewWire(rw io.ReadWriter, maxSize int) *Wire {
	if maxSize < 4 {
		panic("Wire must at least have size of 4")
	}
	return &Wire{newStreamer(rw, maxSize)}
}

func (w *Wire) ReadMessage() (Message, error) {
	length := uint(0)
	if err := w.ReadNumber(&length); err != nil {
		return nil, err
	}
	if length == 0 {
		return KeepAlive{}, nil
	}
	if length > uint(w.max) {
		return nil, errors.New("message is too large")
	}
	hdr, err := w.ReadByte()
	if err != nil {
		return nil, err
	}
	switch hdr {
	case choke:
		return Choke{}, nil
	case unchoke:
		return Unchoke{}, nil
	case interested:
		return Interested{}, nil
	case notInterested:
		return NotInterested{}, nil
	case have:
		hm := Have{}
		if err := w.ReadNumber(&hm.Index); err != nil {
			return nil, err
		}
		return hm, nil
	case request:
		rm := Request{}
		if err := w.ReadNumbers(&rm.Index, &rm.Begin, &rm.Length); err != nil {
			return nil, err
		}
		return rm, nil
	case cancel:
		cm := Cancel{}
		if err := w.ReadNumbers(&cm.Index, &cm.Begin, &cm.Length); err != nil {
			return nil, err
		}
		return cm, nil
	case port:
		pm := Port{}
		if err := w.ReadNumber(&pm.Port); err != nil {
			return nil, err
		}
		return pm, nil
	case bitfield:
		bf, err := bf.BitFieldFromReader(w.ReadWriter, int(length)*8)
		if err != nil {
			return nil, err
		}
		return BitField{bf}, nil
	case piece:
		pm := Piece{0, 0, make([]byte, length)}
		if err := w.ReadNumbers(&pm.Index, &pm.Begin); err != nil {
			return nil, err
		}
		if err := w.ReadRaw(pm.Piece); err != nil {
			return nil, err
		}
		return pm, nil
	}
	return nil, errors.New("could not recognize message header")
}

func (w *Wire) Send(m Message) error { return m.Write(w.streamer) }

func (w *Wire) ReceiveHandshake() (h Handshake, err error) {
	l, err := w.ReadByte()
	if err != nil {
		return h, err
	}
	if int(l) != len(btPreamble) {
		return h, errors.New("handshake length byte is unexpected value")
	}
	preamble, err := w.ReadBytes(len(btPreamble))
	if err != nil {
		return h, err
	}
	if !bytes.Equal(preamble, btPreamble) {
		return h, errors.New("handshake preamble is wrong")
	}
	res, err := bf.BitFieldFromReader(w.streamer, numReservedBits)
	if err != nil {
		return h, err
	}
	if len(res.AllSet()) == 0 {
		h.Extension = Original
	} else if res.IsSet(numReservedBits - 1) {
		h.Extension = DHT
	} else {
		return h, errors.New("unrecognized reserved byte section")
	}
	return h, w.ReadRaws(h.Hash, h.PeerID)
}
