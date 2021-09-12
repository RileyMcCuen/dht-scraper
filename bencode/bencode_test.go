package bencode

import (
	"bytes"
	_ "embed"
	"testing"
)

func TestDecodeString(t *testing.T) {
	input := "4:spam"
	s, err := DecodeFromString(input)
	if err != nil {
		t.Fatal(err)
	}
	ss, ok := s.(String)
	if !ok {
		t.Fatal("decoded value was not a String but should have been")
	}
	if ss.Len() != 4 {
		t.Fatal("String.Len returned the wrong length")
	}
	if ss.Raw() != "spam" {
		t.Fatal("raw string should have been spam but was not")
	}
	if ss.String() != input {
		t.Fatal("String did not bencode correctly")
	}
	if ss.String() != StringFromBytes([]byte("spam")).String() {
		t.Fatal("S and StringFromBytes returned Strings that were not equal")
	}
}

func TestEqualString(t *testing.T) {
	a, b := S("cow"), S("cow")
	if !a.Equal(b) {
		t.Fatal("two String values of cow did not equal each other")
	}
	if !b.Equal(a) {
		t.Fatal("two String values of cow did not equal each other")
	}
	a = S("spa")
	if a.Equal(b) {
		t.Fatal("String values of spa and cow reported that they are equal")
	}
	if b.Equal(a) {
		t.Fatal("String values of spa and cow reported that they are equal")
	}
	a = S("spam")
	if a.Equal(b) {
		t.Fatal("String values of spam and cow reported that they are equal")
	}
	if b.Equal(a) {
		t.Fatal("String values of spam and cow reported that they are equal")
	}
}

func TestLessString(t *testing.T) {
	a, b := S("cow"), S("spam")
	if !a.Less(b) {
		t.Fatal("cow should be less than spam but was not")
	}
	if b.Less(a) {
		t.Fatal("spam should not be less than cow but was")
	}
	a = S("spa")
	if !a.Less(b) {
		t.Fatal("spa should be less than spam but was not")
	}
	if b.Less(a) {
		t.Fatal("spam should not be less than spa but was")
	}
}

func TestDecodeInt(t *testing.T) {
	input := "i131e"
	i, err := DecodeFromString(input)
	if err != nil {
		t.Fatal(err)
	}
	ii, ok := i.(Int)
	if !ok {
		t.Fatal("decoded value was not an Int but should have been")
	}
	if ii.Raw() != 131 {
		t.Fatal("raw int should have been 131 but was not")
	}
	if ii.String() != input {
		t.Fatal("Int did not bencode correctly")
	}
}

func TestDecodeNegativeInt(t *testing.T) {
	input := "i-131e"
	i, err := DecodeFromString(input)
	if err != nil {
		t.Fatal(err)
	}
	ii, ok := i.(Int)
	if !ok {
		t.Fatal("decoded value was not an Int but should have been")
	}
	if ii.Raw() != -131 {
		t.Fatal("raw int should have been -131 but was not")
	}
	if ii.String() != input {
		t.Fatal("Int did not bencode correctly")
	}
}

func TestDecodeList(t *testing.T) {
	input := "l4:spami131ee"
	l, err := DecodeFromString(input)
	if err != nil {
		t.Fatal(err)
	}
	ll, ok := l.(List)
	if !ok {
		t.Fatal("decoded value was not a List but should have been")
	}
	if ll.Get(0).(String).Raw() != "spam" {
		t.Fatal("decoded List had mangled String value at index 0")
	}
	if ll.Get(1).(Int).Raw() != 131 {
		t.Fatal("decoded List had mangled Int value at index 1")
	}
	if ll.Get(2) != nil {
		t.Fatal("List returned non nil value for index out of bounds")
	}
	if ll.String() != input {
		t.Fatal("List did not bencode correctly")
	}
}

func TestDecodeDict(t *testing.T) {
	input := "d4:spami131ee"
	d, err := DecodeFromString(input)
	if err != nil {
		t.Fatal(err)
	}
	dd, ok := d.(Dict)
	if !ok {
		t.Fatal("decoded value was not a Dict but should have been")
	}
	if dd.Get(S("spam")).(Int).Raw() != 131 {
		t.Fatal("Dict does not contain expected key or associated value")
	}
	if dd.Get(S("doesntexist")) != nil {
		t.Fatal("Dict returned non nil value for key that does not exist")
	}
	if dd.String() != input {
		t.Fatal("Dict did not bencode correctly")
	}
}

func equal(a, b []String) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if !a[i].Equal(b[i]) {
			return false
		}
	}
	return true
}

func TestBuildDict(t *testing.T) {
	d := D()
	d = d.Put(S("cow"), S("moo"))
	if !equal(d.Keys(), []String{S("cow")}) {
		t.Fatal("dictionary keys are either missing or in incorrect order")
	}
	d = d.Put(S("all"), S("aboard"))
	if !equal(d.Keys(), []String{S("all"), S("cow")}) {
		t.Fatal("dictionary keys are either missing or in incorrect order")
	}
	d = d.Put(S("bin"), S("man"))
	if !equal(d.Keys(), []String{S("all"), S("bin"), S("cow")}) {
		t.Fatal("dictionary keys are either missing or in incorrect order")
	}
	d = d.Put(S("jazz"), S("cobra"))
	if !equal(d.Keys(), []String{S("all"), S("bin"), S("cow"), S("jazz")}) {
		t.Fatal("dictionary keys are either missing or in incorrect order")
	}
	d = d.Put(S("aaa"), S("bbb"))
	if !equal(d.Keys(), []String{S("aaa"), S("all"), S("bin"), S("cow"), S("jazz")}) {
		t.Fatal("dictionary keys are either missing or in incorrect order")
	}
	d = d.Put(S("albino"), S("rhino"))
	if !equal(d.Keys(), []String{S("aaa"), S("albino"), S("all"), S("bin"), S("cow"), S("jazz")}) {
		t.Fatal("dictionary keys are either missing or in incorrect order")
	}
}

var (
	//go:embed ubuntu-21.04-desktop-amd64.iso.torrent
	realWorldData []byte
	torrent       interface{}
	err           error
	buffer        *bytes.Buffer
)

func TestDecodeRealWorldData(t *testing.T) {
	torrent, err = DecodeFromBytes(realWorldData)
	if err != nil {
		t.Fatal(err, torrent.(Bencoder).String())
	}
	if err = torrent.(Bencoder).Bencode(buffer); err != nil {
		t.Fatal(err)
	}
	if string(realWorldData) != buffer.String() {
		t.Fatal("did not equivalent but should have been")
	}
}

func BenchmarkRealWorld(b *testing.B) {
	b.SetParallelism(1)
	b.ReportAllocs()
	b.Run("Unmarshal", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			torrent, err = DecodeFromBytes(realWorldData)
			if err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
	})
	b.Run("Marshal", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			buffer = bytes.NewBuffer(mbytes(0))
			if err = torrent.(Bencoder).Bencode(buffer); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
	})
	b.StopTimer()
	if string(realWorldData) != buffer.String() {
		b.Fatal("strings should have been equal but were not")
	}
}
