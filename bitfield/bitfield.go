package bitfield

import (
	"errors"
	"fmt"
	"io"
)

type (
	BitField struct {
		numPieces int
		pieces    []byte
	}
)

const (
	off   byte = 0
	on    byte = 128
	allOn byte = 255
)

func bitFull(b byte, bit int) bool { return b&(on>>bit) > 0 }

func NewBitField(numPieces int, full bool) *BitField {
	if numPieces <= 0 {
		panic("cannot have bitfield of size <= 0")
	}
	ret := &BitField{
		numPieces,
		make([]byte, (numPieces+7)/8),
	}
	if full {
		ret.Fill()
	}
	return ret
}

func BitFieldFromReader(r io.Reader, numPieces int) (*BitField, error) {
	size := (numPieces + 7) / 8
	ret := &BitField{
		numPieces,
		make([]byte, size),
	}
	n, err := r.Read(ret.pieces)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if n != size {
		return nil, errors.New("could not recognize message header")
	}
	return ret, nil
}

func (b *BitField) String() string {
	ret := ""
	for _, by := range b.pieces {
		ret += fmt.Sprintf("%08b", by)
	}
	return ret
}

func (b *BitField) Fill() *BitField {
	fullPieces, specialLastByte := b.pieces, b.numPieces%8 != 0
	if specialLastByte {
		fullPieces = b.pieces[:len(fullPieces)-1]
	}
	for i := range fullPieces {
		b.pieces[i] = allOn
	}
	if specialLastByte {
		by, bit := b.pieces[len(b.pieces)-1], on
		for i := 0; i < b.numPieces%8; i++ {
			by |= bit
			bit >>= 1
		}
		b.pieces[len(b.pieces)-1] = by
	}
	return b
}

func (b *BitField) Clear() *BitField {
	for i := range b.pieces {
		b.pieces[i] = off
	}
	return b
}

func (b *BitField) IsSet(i int) bool {
	byIdx, bitIdx := i/8, i%8
	return bitFull(b.pieces[byIdx], bitIdx)
}

func (b *BitField) Set(i int) *BitField {
	byIdx, bitIdx := i/8, i%8
	b.pieces[byIdx] = b.pieces[byIdx] | on>>bitIdx
	return b
}

func (b *BitField) Unset(i int) *BitField {
	byIdx, bitIdx := i/8, i%8
	b.pieces[byIdx] = b.pieces[byIdx] & ^(on >> bitIdx)
	return b
}

func (b *BitField) Next(start int) int {
	for i, by := range b.pieces {
		if by == allOn {
			continue
		}
		for j := 0; j < 8 && (i*8)+j < b.numPieces; j++ {
			if !bitFull(by, j) {
				return (i * 8) + j
			}
		}
	}
	return -1
}

func (b *BitField) AllSet() []int {
	ret := make([]int, 0)
	for i, by := range b.pieces {
		if by == allOn {
			for j := 0; j < 8 && (i*8)+j < b.numPieces; j++ {
				ret = append(ret, (i*8)+j)
			}
		} else {
			for j := 0; j < 8 && (i*8)+j < b.numPieces; j++ {
				if bitFull(by, j) {
					ret = append(ret, (i*8)+j)
				}
			}
		}
	}
	return ret
}

func (b *BitField) AllUnset() []int {
	ret := make([]int, 0)
	for i, by := range b.pieces {
		if by == allOn {
			continue
		}
		for j := 0; j < 8 && (i*8)+j < b.numPieces; j++ {
			if !bitFull(by, j) {
				ret = append(ret, (i*8)+j)
			}
		}
	}
	return ret
}

func (b *BitField) NumBytes() int { return len(b.pieces) }

func (b *BitField) Bytes() []byte { return b.pieces }
