package txcar

import (
	"encoding/binary"
	"io"
)

type ZoReader struct {
	*io.LimitedReader
}

func NewZoReader(size uint64) io.Reader {
	return &ZoReader{(io.LimitReader(NewUnlimitedZoReader(), int64(size))).(*io.LimitedReader)}
}

type UnlimitedZoReader struct {
	Pos uint64
}

func NewUnlimitedZoReader() io.Reader {
	return &UnlimitedZoReader{
		Pos: 0,
	}
}

func (r *UnlimitedZoReader) Read(out []byte) (int, error) {
	prePad := (-r.Pos) % 8
	n := r.Pos / 8
	bufLeft := uint64(len(out))
	bufIndex := uint64(0)
	tempBytes := make([]byte, 8)
	//fmt.Printf("bufLeft:%d\tbufIndex:%d\tr.Pos:%d\tn:%d\t\n", bufLeft, bufIndex, r.Pos, n)

	// prePad
	if prePad > 0 {
		if bufLeft < prePad {
			binary.LittleEndian.PutUint64(tempBytes, n)
			copy(out, tempBytes[8-prePad:8-prePad+bufLeft])
			r.Pos += bufLeft
			return len(out), nil
		}
		binary.LittleEndian.PutUint64(tempBytes, n)
		copy(out, tempBytes[8-prePad:])
		r.Pos += prePad
		bufLeft -= prePad
		bufIndex += prePad
		n += 1
		//fmt.Printf("1  bufLeft:%d\tbufIndex:%d\tr.Pos:%d\tn:%d\t\n", bufLeft, bufIndex, r.Pos, n)
	}

	// regular
	count := bufLeft / 8
	for i := uint64(0); i < count; i++ {
		//fmt.Printf("2  bufLeft:%d\tbufIndex:%d\tr.Pos:%d\tn:%d\t\n", bufLeft, bufIndex, r.Pos, n)
		binary.LittleEndian.PutUint64(out[bufIndex:], n)
		r.Pos += 8
		bufLeft -= 8
		bufIndex += 8
		n += 1
	}

	// suffix
	if bufLeft > 0 {
		//fmt.Printf("3  bufLeft:%d\tbufIndex:%d\tr.Pos:%d\tn:%d\t\n", bufLeft, bufIndex, r.Pos, n)
		binary.LittleEndian.PutUint64(tempBytes, n)
		copy(out[bufIndex:], tempBytes[0:bufLeft])
		r.Pos += bufLeft
	}

	return len(out), nil
}
