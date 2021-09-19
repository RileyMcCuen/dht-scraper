package dht

import (
	"bytes"
	"dht/bencode"
	"fmt"
	"testing"
)

func TestNode(t *testing.T) {
	input := []byte{70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 90, 90, 90, 90, 65, 65}
	n := ParseNode(input)
	if !n.Valid(bencode.S("")) {
		t.Fatal("node was not valid but should have been")
	}
	if !bytes.Equal(n.ID, []byte{70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79}) {
		t.Fatal("node has an unexpected id")
	}
	if !n.IP.Equal([]byte{90, 90, 90, 90}) {
		t.Fatal("node has an unexpected ip address")
	}
	if n.port != 16705 {
		t.Fatal("node has an unexpected port", n.port)
	}
	if string(input) != n.String() {
		fmt.Println(string(input))
		fmt.Println(n.String())
		t.Fatal("output of node.String was not the same as input")
	}
}
