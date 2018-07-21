package udp

import (
	"encoding/binary"
)

const (
	fecHeaderSize = 4
)

type (
	// fecDecoder for decoding incoming packets
	fecDecoder struct {
		seqlen uint32
		seqs   []uint32
	}
)

func newFECDecoder(seqlen uint32) *fecDecoder {
	if seqlen <= 0 {
		return nil
	}

	dec := new(fecDecoder)
	dec.seqlen = seqlen
	dec.seqs = make([]uint32, seqlen)
	return dec
}

// decodeBytes a fec packet
func (dec *fecDecoder) decodeBytes(data []byte) []byte {
	seqid := binary.LittleEndian.Uint32(data)
	index := seqid % dec.seqlen
	if dec.seqs[index] == seqid {
		return nil
	}
	dec.seqs[index] = seqid
	return data[fecHeaderSize:]
}

type (
	// fecEncoder for encoding outgoing packets
	fecEncoder struct {
		seqid uint32
		queue [][]byte
	}
)

func newFECEncoder(queuelen, bufsize int) *fecEncoder {
	if queuelen <= 1 {
		return nil
	}
	enc := new(fecEncoder)
	enc.seqid = 0
	enc.queue = make([][]byte, queuelen)
	for i := 0; i < queuelen; i++ {
		enc.queue[i] = make([]byte, bufsize)[:0]
	}
	return enc
}

func (enc *fecEncoder) encode(b []byte) (ps [][]byte) {
	qend := enc.queue[len(enc.queue)-1][:len(b)]
	for i := len(enc.queue) - 1; i > 0; i-- {
		enc.queue[i] = enc.queue[i-1]
	}

	enc.seqid++
	binary.LittleEndian.PutUint32(qend, enc.seqid)
	copy(qend[fecHeaderSize:fecHeaderSize+len(b)], b)
	enc.queue[0] = qend[:fecHeaderSize+len(b)]

	return enc.queue
}
