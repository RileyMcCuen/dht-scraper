package bencode

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
)

type (
	stringReadCloser struct {
		io.Reader
		read int
	}
	pair struct {
		Key   String
		Value Bencoder
	}
	String   []byte     // <num bytes>:<bytes>
	Int      int64      // i<num>e
	List     []Bencoder // l<elems>e
	Dict     []pair     // d<keyvalues>e
	Bencoder interface {
		Bencode(data []byte) []byte
		String() string
	}
)

// Helpers

func mbytes(sz int) []byte { return make([]byte, sz) }

func (r *stringReadCloser) Close() error { return nil }
func (r *stringReadCloser) Read(p []byte) (int, error) {
	i, err := r.Reader.Read(p)
	r.read += i
	return i, err
}

func readUntil(r io.Reader, b byte, app []byte) ([]byte, error) {
	buf := mbytes(1)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if buf[0] == b {
				return app, nil
			} else {
				app = append(app, buf[0])
			}
		} else if err != nil {
			if err == io.EOF {
				return app, errors.New("eof reached before byte was found")
			}
			return app, err
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

func (s String) Bencode(data []byte) []byte {
	data = strconv.AppendInt(data, int64(len(s)), 10)
	data = append(data, stringSep)
	return append(data, s...)
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

func (s String) String() string { return string(s.Bencode(mbytes(0))) }

func I(i int64) Int { return Int(i) }

func (i Int) Bencode(data []byte) []byte {
	data = append(data, intStart)
	data = strconv.AppendInt(data, int64(i), 10)
	return append(data, end)
}

func (i Int) Raw() int64 { return int64(i) }

func (i Int) String() string { return string(i.Bencode(mbytes(0))) }

func L(elems ...Bencoder) List { return List(elems) }

func (l List) Bencode(data []byte) []byte {
	data = append(data, listStart)
	for _, elem := range l {
		data = elem.Bencode(data)
	}
	return append(data, end)
}

func (l List) Append(elem Bencoder) List { return append(l, elem) }

func (l List) Get(idx int) Bencoder {
	if idx < len(l) {
		return l[idx]
	}
	return nil
}

func (l List) String() string { return string(l.Bencode(mbytes(0))) }

func p(k String, v Bencoder) pair { return pair{k, v} }

func (p pair) bencode(data []byte) []byte {
	data = p.Key.Bencode(data)
	data = p.Value.Bencode(data)
	return data
}

func D(pairs ...pair) Dict { return Dict(pairs) }

func (d Dict) Bencode(data []byte) []byte {
	data = append(data, dictStart)
	for _, val := range d {
		data = val.bencode(data)
	}
	return append(data, end)
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
	ret[idx] = p(k, v)
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

func (d Dict) String() string { return string(d.Bencode(mbytes(0))) }

// Functions to handle decoding of types coming over the wire

func decodeString(r io.Reader, fByte []byte) (String, error) {
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
	n, err := r.Read(s)
	if int64(n) == strLen {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, errors.New("full string could not be read")
}

func decodeInt(r io.Reader) (Int, error) {
	data, err := readUntil(r, end, mbytes(0))
	if err != nil {
		return 0, err
	}
	i, err := parseInt(data)
	return Int(i), err
}

func decodeList(r io.Reader) (List, error) {
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

func decodeDict(r io.Reader) (Dict, error) {
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
			d = append(d, p(ks, v))
		} else {
			return d, err
		}
	}
}

func decode(r io.Reader) (Bencoder, error) {
	// TODO: Chunk stream -> speed up reads from pipe / less allocation
	b := mbytes(1)
	n, err := r.Read(b)
	if n > 0 {
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
			if rr, ok := r.(*stringReadCloser); ok {
				return nil, errors.New("invalid character " + string(b[0]) + " found at " + fmt.Sprint(rr.read))
			}
			return nil, errors.New("invalid character found " + string(b[0]))
		}
	} else if err != nil {
		return nil, err
	}
	return nil, errors.New("empty reader cannot be decoded")
}

func Decode(r io.ReadCloser) (Bencoder, error) {
	defer r.Close()
	ret, err := decode(r)
	if err == io.EOF {
		return ret, nil
	}
	return ret, err
}

func DecodeFromBytes(bs []byte) (Bencoder, error) {
	return Decode(&stringReadCloser{bytes.NewBuffer(bs), 0})
}

func DecodeFromString(s string) (Bencoder, error) {
	return DecodeFromBytes([]byte(s))
}
