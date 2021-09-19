package bencode

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"sort"
	"strconv"
	"strings"
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
	ReadWriter struct {
		*Reader
		*Writer
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
		Unmarshal(dst reflect.Value) error
		Pretty(ind, indInc string) string
		Bytes() []byte
		String() string
	}
)

// Helpers

func NewReader(r io.Reader) *Reader { return &Reader{r, 0, 0, nil} }
func NewWriter(w io.Writer) *Writer { return &Writer{w, 0, nil} }
func NewReadWriter(rw io.ReadWriter) *ReadWriter {
	return &ReadWriter{NewReader(rw), NewWriter(rw)}
}

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

func (s String) Unmarshal(dst reflect.Value) error {
	if dst.Kind() != reflect.Ptr {
		return errors.New("(String) can only unmarshal to ptr")
	}
	e := dst.Elem()
	if !e.CanSet() {
		return errors.New("(String) cannot set field")
	}
	if e.Type() == reflect.TypeOf(S("")) {
		e.Set(reflect.ValueOf(s))
	} else {
		switch k := e.Kind(); {
		case k == reflect.String:
			e.SetString(s.Raw())
		case k == reflect.Slice && e.Elem().Kind() == reflect.Int8:
			e.SetBytes(s)
		default:
			return errors.New("(String) invalid type for field")
		}
	}
	return nil
}

func (s String) Pretty(ind, _ string) string {
	if s.Len() <= 4 {
		return ind + string(s) + "\n"
	}
	return ind + s.Raw() + "\n"
}
func (s String) Raw() string { return string(s) }

func (s String) Equal(o String) bool {
	return bytes.Equal(s, o)
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

func (i Int) Unmarshal(dst reflect.Value) error {
	if dst.Kind() != reflect.Ptr {
		return errors.New("(Int) can only unmarshal to ptr")
	}
	e := dst.Elem()
	if !e.CanSet() {
		return errors.New("(Int) cannot set field")
	}
	if e.Type() == reflect.TypeOf(I(0)) {
		e.Set(reflect.ValueOf(i))
	} else {
		switch k := e.Kind(); k {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			e.SetInt(i.Raw())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			e.SetUint(uint64(i.Raw()))
		default:
			return errors.New("(Int) invalid type for field")
		}
	}
	return nil
}

func (i Int) Pretty(ind, _ string) string { return ind + fmt.Sprint(i.Raw()) + "\n" }
func (i Int) Raw() int64                  { return int64(i) }

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

func (l List) Unmarshal(dst reflect.Value) error {
	if dst.Kind() != reflect.Ptr {
		return errors.New("(List) can only unmarshal to ptr")
	}
	e := dst.Elem()
	if e.Type() == reflect.TypeOf(L()) {
		if !e.CanSet() {
			return errors.New("(List) cannot set field")
		}
		e.Set(reflect.ValueOf(l))
	} else {
		switch k := e.Kind(); k {
		case reflect.Struct:
			if e.NumField() != l.Len() {
				return errors.New("(List) struct does not have enough fields")
			}
			for i, val := range l {
				if err := val.Unmarshal(e.Field(i).Addr()); err != nil {
					return err
				}
			}
		default:
			return errors.New("(List) invalid type for field")
		}
	}
	return nil
}

func (l List) Pretty(ind, indInc string) string {
	ret := strings.Builder{}
	for _, elem := range l {
		ret.WriteString(elem.Pretty(ind, indInc))
	}
	return ret.String()
}

func (l List) Append(elem Bencoder) List { return append(l, elem) }

func (l List) Get(idx int) Bencoder {
	if idx < len(l) {
		return l[idx]
	}
	return nil
}

func (l List) Len() int { return len(l) }

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

func (d Dict) Pretty(ind, indInc string) string {
	nextInd, ret := ind+indInc, strings.Builder{}
	for _, p := range d {
		k, v := p.Key, p.Value
		ret.WriteString(k.Pretty(ind, indInc))
		ret.WriteString(v.Pretty(nextInd, indInc))
	}
	return ret.String()
}

func (d Dict) Bencode(w *Writer) error {
	w.Write(btobs(dictStart))
	for _, val := range d {
		val.bencode(w)
	}
	w.Write(btobs(end))
	return w.err
}

func (d Dict) Unmarshal(dst reflect.Value) error {
	if dst.Kind() != reflect.Ptr {
		return errors.New("(Dict) can only unmarshal to ptr")
	}
	e := dst.Elem()
	if e.Type() == reflect.TypeOf(D()) {
		if !e.CanSet() {
			return errors.New("(Dict) cannot set field")
		}
		e.Set(reflect.ValueOf(d))
	} else {
		switch k := e.Kind(); k {
		case reflect.Struct:
			if e.NumField() != d.Len() {
				return errors.New("(Dict) struct does not have enough fields")
			}
			for i, val := range d {
				if err := val.Value.Unmarshal(e.Field(i).Addr()); err != nil {
					return err
				}
			}
		default:
			return errors.New("Dict) invalid type for field")
		}
	}
	return nil
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
	if i >= d.Len() || i < 0 {
		return nil
	}
	if pr := d[i]; pr.Key.Equal(k) {
		return pr.Value
	}
	return nil
}

func (d Dict) Len() int { return len(d) }

func (d Dict) GetString(k String) (String, error) {
	ret := d.Get(k)
	if ret == nil {
		return nil, errors.New("no such key in dict")
	}
	if retStr, ok := ret.(String); ok {
		return retStr, nil
	}
	return nil, errors.New("value for key in dict was not a String")
}

func (d Dict) GetInt(k String) (Int, error) {
	ret := d.Get(k)
	if ret == nil {
		return 0, errors.New("no such key in dict")
	}
	if retInt, ok := ret.(Int); ok {
		return retInt, nil
	}
	return 0, errors.New("value for key in dict was not a Int")
}

func (d Dict) GetList(k String) (List, error) {
	ret := d.Get(k)
	if ret == nil {
		return nil, errors.New("no such key in dict")
	}
	if retList, ok := ret.(List); ok {
		return retList, nil
	}
	return nil, errors.New("value for key in dict was not a List")
}

func (d Dict) GetDict(k String) (Dict, error) {
	ret := d.Get(k)
	if ret == nil {
		return nil, errors.New("no such key in dict")
	}
	if retDict, ok := ret.(Dict); ok {
		return retDict, nil
	}
	return nil, errors.New("value for key in dict was not a String")
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
	} else {
		log.Println("len", strLen, r.n)
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
