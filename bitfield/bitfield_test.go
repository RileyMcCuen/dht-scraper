package bitfield

import (
	"fmt"
	"testing"
)

func TestBitField(t *testing.T) {
	for _, l := range []int{13, 16} {
		bf := NewBitField(l, false)
		if len(bf.AllUnset()) != l {
			t.Fatal("bitfield returned wrong amount of missing pieces")
		}
		if len(bf.AllSet()) != 0 {
			t.Fatal("bitfield returned wrong amount of owned pieces")
		}
		if bf.IsSet(0) {
			t.Fatal("bitfield incorrectly reported having bit that it should not")
		}
		bf.Set(0)
		if !bf.IsSet(0) {
			t.Fatal("bitfield incorrectly reported not having bit that it should")
		}
		if bf.Next(0) != 1 {
			t.Fatal("bitfield reported wrong nextbit")
		}
		bf.Unset(0)
		if bf.IsSet(0) {
			t.Fatal("bitfield incorrectly reported having bit that it should not")
		}
		bf.Fill()
		if len(bf.AllUnset()) != 0 {
			fmt.Println(len(bf.AllUnset()))
			t.Fatal("bitfield returned wrong amount of missing pieces")
		}
		if len(bf.AllSet()) != l {
			t.Fatal("bitfield returned wrong amount of owned pieces")
		}
		bf.Clear()
		if len(bf.AllSet()) != 0 {
			t.Fatal("bitfield returned wrong amount of owned pieces")
		}
	}
}
