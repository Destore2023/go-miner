package disk

import (
	"errors"
	"os"
)

const (
	MaxBlockFileSize   = 128 * 1024 * 1024 // 128 MiB
	BlockFileChunkSize = 16 * 1024 * 1024  // 16 MiB

	MinDiskSpace    = 256 * 1024 * 1024 // 256 MiB
	BlockFilePrefix = "blk"
)

var (
	ErrInvalidFlatFilePos    = errors.New("invalid FlatFilePos")
	ErrOutOfSpace            = errors.New("out of space")
	ErrReadBrokenBlockHeader = errors.New("read broken block header")
	ErrReadBrokenBlock       = errors.New("read broken block")
	ErrReadBrokenData        = errors.New("read broken data")
	ErrFileOutOfRange        = errors.New("file out of range")
	ErrClosed                = errors.New("file writer closed")
)

var (
	MagicNo                  = [4]byte{0xf9, 0xbe, 0xb4, 0xd9}
	MagicNoLength            = len(MagicNo)
	RawBlockSizeLength       = 8
	BlockMessageHeaderLength = MagicNoLength + RawBlockSizeLength
)

var CheckDiskSpaceStub func(path string, additional uint64) bool

func TruncateFile(f *os.File, size int64) error {
	return f.Truncate(size)
}
