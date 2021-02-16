package disk

import "fmt"

type FlatFilePos struct {
	fileNo uint32 // Block File Number (4 bytes)
	pos    int64  // Position          (8 bytes)
}

func NewFlatFilePos(fileNo uint32, pos int64) *FlatFilePos {
	return &FlatFilePos{
		fileNo: fileNo,
		pos:    pos,
	}
}

func (f *FlatFilePos) String() string {
	return fmt.Sprintf("FlatFilePos=%05d:%d", f.fileNo, f.pos)
}

func (f *FlatFilePos) FileNo() uint32 {
	return f.fileNo
}

func (f *FlatFilePos) Pos() int64 {
	return f.pos
}
