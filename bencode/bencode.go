package bencode

import (
	"bytes"
	"errors"
	"io"
	"sort"
	"strconv"
)

type (
	Reader struct {
		io.Reader
		n, read int64
		err     error
	}
	Writer struct {
		io.Writer
		writ int64
		err  error
	}
	Pair struct {
		Key   String
		Value Bencoder
	}
	String   []byte     // <num bytes>:<bytes>
	Int      int64      // i<num>e
	List     []Bencoder // l<elems>e
	Dict     []Pair     // d<keyvalues>e
	Bencoder interface {
		Bencode(*Writer) error
		Bytes() []byte
		String() string
	}
)

// Helpers

func NewReader(r io.Reader) *Reader { return &Reader{r, 0, 0, nil} }
func NewWriter(w io.Writer) *Writer { return &Writer{w, 0, nil} }

func (r *Reader) Error() error      { return r.err }
func (w *Writer) Error() error      { return w.err }
func (r *Reader) NumRead() int64    { return r.read }
func (w *Writer) NumWritten() int64 { return w.writ }

func (r *Reader) Read(p []byte) *Reader {
	if r.err == nil {
		i, err := r.Reader.Read(p)
		r.n, r.read, r.err = int64(i), r.read+int64(i), err
	}
	return r
}
func (w *Writer) Write(p []byte) *Writer {
	if w.err == nil {
		i, err := w.Writer.Write(p)
		w.writ, w.err = w.writ+int64(i), err
	}
	return w
}

func mbytes(sz int) []byte { return make([]byte, sz) }
func btobs(b byte) []byte  { return []byte{b} }

func readUntil(r *Reader, b byte, app []byte) ([]byte, error) {
	buf := mbytes(1)
	for {
		r.Read(buf)
		if r.n > 0 {
			if buf[0] == b {
				return app, nil
			} else {
				app = append(app, buf[0])
			}
		} else if r.err != nil {
			if r.err == io.EOF {
				return app, errors.New("eof reached before byte was found")
			}
			return app, r.err
		} else {
			return app, errors.New("empty reader cannot be readUntil")
		}
	}
}

func parseInt(bs []byte) (int64, error) {
	var val, mult, sign int64 = 0, 1, 1
	if bs[0] == neg {
		sign = -1
		bs = bs[1:]
	}
	if len(bs) == 0 {
		return 0, errors.New("input was too short to be a valid number")
	} else if len(bs) == 1 {
		if bs[0] == num0 {
			if sign == -1 {
				return 0, errors.New("-0 is not a valid number")
			}
			return 0, nil // special case: num == 0
		}
	} else if bs[0] == num0 {
		return 0, errors.New("numbers aside from 0 with leading 0s are not valid numbers")
	}
	for i := len(bs) - 1; i >= 0; i-- {
		switch b := bs[i]; b {
		case num0, num1, num2, num3, num4, num5, num6, num7, num8, num9:
			val += (int64(b-num0) * mult)
			mult *= 10
		default:
			return 0, errors.New("invalid character found when parsing int")
		}
	}
	return sign * val, nil
}

const (
	neg       = '-'
	num0      = '0'
	num1      = '1'
	num2      = '2'
	num3      = '3'
	num4      = '4'
	num5      = '5'
	num6      = '6'
	num7      = '7'
	num8      = '8'
	num9      = '9'
	stringSep = ':'
	end       = 'e'
	intStart  = 'i'
	listStart = 'l'
	dictStart = 'd'
)

// Implementations of types that can come over the wire

func S(s string) String               { return String(s) }
func StringFromBytes(s []byte) String { return String(s) }

func (s String) Bencode(w *Writer) error {
	w.
		Write(strconv.AppendInt(mbytes(0), int64(len(s)), 10)).
		Write(btobs(stringSep)).
		Write(s)
	return w.err
}

func (s String) Raw() string { return string(s) }

func (s String) Equal(o String) bool {
	if len(s) != len(o) {
		return false
	}
	for i, sc := range s {
		oc := o[i]
		if sc != oc {
			return false
		}
	}
	return true
}

func (s String) Less(o String) bool {
	for i := 0; i < len(s) && i < len(o); i++ {
		sc, oc := s[i], o[i]
		if sc < oc {
			return true
		} else if oc < sc {
			return false
		}
	}
	return len(s) < len(o)
}

func (s String) Len() int { return len(s) }

func (s String) Bytes() []byte {
	buf := NewWriter(bytes.NewBuffer(mbytes(0)))
	s.Bencode(buf)
	return buf.Writer.(*bytes.Buffer).Bytes()
}

func (s String) String() string { return string(s.Bytes()) }

func I(i int64) Int { return Int(i) }

func (i Int) Bencode(w *Writer) error {
	w.
		Write(btobs(intStart)).
		Write(strconv.AppendInt(mbytes(0), int64(i), 10)).
		Write(btobs(end))
	return w.err
}

func (i Int) Raw() int64 { return int64(i) }

func (i Int) Bytes() []byte {
	buf := NewWriter(bytes.NewBuffer(mbytes(0)))
	i.Bencode(buf)
	return buf.Writer.(*bytes.Buffer).Bytes()
}

func (i Int) String() string { return string(i.Bytes()) }

func L(elems ...Bencoder) List { return List(elems) }

func (l List) Bencode(w *Writer) error {
	w.Write(btobs(listStart))
	for _, elem := range l {
		elem.Bencode(w)
	}
	w.Write(btobs(end))
	return w.err
}

func (l List) Append(elem Bencoder) List { return append(l, elem) }

func (l List) Get(idx int) Bencoder {
	if idx < len(l) {
		return l[idx]
	}
	return nil
}

func (l List) Bytes() []byte {
	buf := NewWriter(bytes.NewBuffer(mbytes(0)))
	l.Bencode(buf)
	return buf.Writer.(*bytes.Buffer).Bytes()
}

func (l List) String() string { return string(l.Bytes()) }

func P(k String, v Bencoder) Pair { return Pair{k, v} }

func (p Pair) bencode(w *Writer) error {
	p.Key.Bencode(w)
	return p.Value.Bencode(w)
}

func D(pairs ...Pair) Dict {
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Key.Less(pairs[j].Key)
	})
	return Dict(pairs)
}

func (d Dict) Bencode(w *Writer) error {
	w.Write(btobs(dictStart))
	for _, val := range d {
		val.bencode(w)
	}
	w.Write(btobs(end))
	return w.err
}

func (d Dict) Keys() []String {
	ret := make([]String, 0)
	for _, pr := range d {
		ret = append(ret, pr.Key)
	}
	return ret
}

func (d Dict) IndexOf(k String) int {
	return sort.Search(len(d), func(i int) bool {
		return !d[i].Key.Less(k)
	})
}

func (d Dict) Put(k String, v Bencoder) Dict {
	idx := d.IndexOf(k)
	ret := make(Dict, len(d)+1)
	copy(ret, d[:idx])
	ret[idx] = P(k, v)
	copy(ret[idx+1:], d[idx:])
	return ret
}

func (d Dict) Get(k String) Bencoder {
	i := d.IndexOf(k)
	if pr := d[i]; pr.Key.Equal(k) {
		return pr.Value
	}
	return nil
}

func (d Dict) Bytes() []byte {
	buf := NewWriter(bytes.NewBuffer(mbytes(0)))
	d.Bencode(buf)
	return buf.Writer.(*bytes.Buffer).Bytes()
}

func (d Dict) String() string { return string(d.Bytes()) }

// Functions to handle decoding of types coming over the wire

func decodeString(r *Reader, fByte []byte) (String, error) {
	strLenRaw, err := readUntil(r, stringSep, fByte)
	if err != nil {
		return nil, err
	}
	strLen, err := parseInt(strLenRaw)
	if err != nil {
		return nil, err
	}
	if strLen < 0 {
		return nil, errors.New("string length must be positive or 0")
	}
	s := make(String, strLen)
	r.Read(s)
	if int64(r.n) == strLen {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, errors.New("full string could not be read")
}

func decodeInt(r *Reader) (Int, error) {
	data, err := readUntil(r, end, mbytes(0))
	if err != nil {
		return 0, err
	}
	i, err := parseInt(data)
	return Int(i), err
}

func decodeList(r *Reader) (List, error) {
	l := make(List, 0)
	for {
		b, err := decode(r)
		if b == nil && err == nil {
			return l, nil
		} else if err == nil {
			l = append(l, b)
		} else {
			return l, err
		}
	}
}

func decodeDict(r *Reader) (Dict, error) {
	d := make(Dict, 0)
	for {
		k, err := decode(r)
		if k == nil && err == nil {
			return d, nil
		} else if err != nil {
			return d, err
		}
		ks, ok := k.(String)
		if !ok {
			return d, errors.New("dict key was not a string but should have been")
		}
		v, err := decode(r)
		if v == nil && err == nil {
			return d, errors.New("last dict key has no associated value")
		} else if err == nil {
			d = append(d, P(ks, v))
		} else {
			return d, err
		}
	}
}

func decode(r *Reader) (Bencoder, error) {
	// TODO: Chunk stream -> speed up reads from pipe / less allocation
	b := mbytes(1)
	r.Read(b)
	if r.n > 0 {
		switch b[0] {
		case num0, num1, num2, num3, num4, num5, num6, num7, num8, num9:
			return decodeString(r, b)
		case intStart:
			return decodeInt(r)
		case listStart:
			return decodeList(r)
		case dictStart:
			return decodeDict(r)
		case end:
			return nil, nil
		default:
			return nil, errors.New("invalid character found " + string(b[0]))
		}
	} else if r.err != nil {
		return nil, r.err
	}
	return nil, errors.New("empty reader cannot be decoded")
}

func Decode(r io.Reader) (Bencoder, error) {
	ret, err := decode(NewReader(r))
	if err == io.EOF {
		return ret, nil
	}
	return ret, err
}

func DecodeFromBytes(bs []byte) (Bencoder, error) {
	return Decode(bytes.NewBuffer(bs))
}

func DecodeFromString(s string) (Bencoder, error) {
	return DecodeFromBytes([]byte(s))
}
