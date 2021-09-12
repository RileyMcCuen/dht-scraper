package bittorrent

type (
	msgHeader byte
)

const (
	choke msgHeader = iota
	unchoke
	interested
	notInterested
	have
	bitfield
	request
	piece
	cancel
)
