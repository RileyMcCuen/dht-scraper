package scraper

import (
	"crypto/rand"
	"crypto/sha1"
	"errors"
)

type (
	msgHeader byte
)

const (
	numRandomBytesForID           = 20
	choke               msgHeader = iota
	unchoke
	interested
	notInterested
	have
	bitfield
	request
	piece
	cancel
)

func RandID() ([]byte, error) {
	randBytes := make([]byte, numRandomBytesForID)
	n, err := rand.Read(randBytes)
	if err != nil {
		return nil, err
	}
	if n != numRandomBytesForID {
		return nil, errors.New("not enough random bytes were generated")
	}
	hash := sha1.New()
	if _, err = hash.Write(randBytes); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}
