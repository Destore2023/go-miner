package disk

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/Sukhavati-Labs/go-miner/logging"
)

type BlockFileKeeper struct {
	mu sync.RWMutex

	flatFileSeq *FlatFileSeq

	lastBlockFile uint32 // last block file number
	blockFiles    []*BlockFile
	closed        bool // closed
}

func NewBlockFileKeeper(dir string, records [][]byte) *BlockFileKeeper {
	keeper := &BlockFileKeeper{
		flatFileSeq:   NewFlatFileSeq(dir, BlockFilePrefix, BlockFileChunkSize),
		blockFiles:    make([]*BlockFile, len(records)),
		lastBlockFile: uint32(len(records) - 1),
		closed:        false,
	}
	for i, data := range records {
		readonly := i < len(records)-1
		bf := NewBlockFileFromBytes(data, readonly)
		if bf == nil {
			logging.CPrint(logging.ERROR, "init block file failed", logging.LogFormat{"i": i, "data": data})
			return nil
		}
		keeper.blockFiles[bf.fileNo] = bf

		// check file exist
		if i < len(records)-1 {
			exist, err := keeper.flatFileSeq.ExistFile(NewFlatFilePos(bf.fileNo, 0))
			if err != nil {
				logging.CPrint(logging.ERROR, fmt.Sprintf("check block%05d.dat existence error", bf.fileNo), logging.LogFormat{"err": err})
				return nil
			}
			if !exist {
				logging.CPrint(logging.ERROR, fmt.Sprintf("block%05d.dat missing", bf.fileNo), logging.LogFormat{})
				return nil
			}
		}

	}
	for fileNo, bf := range keeper.blockFiles {
		if bf == nil {
			logging.CPrint(logging.ERROR, fmt.Sprintf("record block%05d missing", fileNo), logging.LogFormat{})
			return nil
		}
	}
	return keeper
}

func (b *BlockFileKeeper) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.flushBlockFile(false)
	for i, bf := range b.blockFiles {
		if i > int(b.lastBlockFile) {
			break
		}
		bf.Close()
	}
	b.closed = true
}

func (b *BlockFileKeeper) flushBlockFile(finalize bool) {
	if len(b.blockFiles) > int(b.lastBlockFile) {
		b.blockFiles[b.lastBlockFile].Flush(int64(b.blockFiles[b.lastBlockFile].Size()), finalize)
	}
}

func (b *BlockFileKeeper) DiscardRecentChange() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blockFiles[b.lastBlockFile].discardRecentChange()
}

func (b *BlockFileKeeper) CommitRecentChange() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blockFiles[b.lastBlockFile].commitRecentChange()
}

func (b *BlockFileKeeper) SaveRawBlockToDisk(rawBlock []byte, height uint64, timestamp int64) (file *BlockFile, offset int64, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, 0, ErrClosed
	}

	if len(rawBlock) == 0 {
		return nil, 0, nil
	}
	blockSize := len(rawBlock)                      // serialized block size
	msgSize := BlockMessageHeaderLength + blockSize // disk data size
	pos, err := b.findBlockPos(height, uint64(msgSize))
	if err != nil {
		return nil, 0, err
	}
	buf := make([]byte, msgSize)
	copy(buf[0:MagicNoLength], MagicNo[:])
	binary.LittleEndian.PutUint64(buf[MagicNoLength:BlockMessageHeaderLength], uint64(blockSize))
	copy(buf[BlockMessageHeaderLength:], rawBlock)

	err = b.blockFiles[pos.FileNo()].WriteRawBlock(b.flatFileSeq, pos.Pos(), buf)
	if err != nil {
		return nil, 0, err
	}
	b.blockFiles[pos.FileNo()].AddBlock(height, uint64(msgSize), uint64(timestamp))
	return b.blockFiles[pos.FileNo()], pos.Pos(), nil
}

func (b *BlockFileKeeper) findBlockPos(height, blockSize uint64) (*FlatFilePos, error) {
	fileNo := b.lastBlockFile
	fileSize := b.blockFiles[fileNo].Size()

	if fileSize+blockSize >= MaxBlockFileSize {
		fileNo++
	}

	if fileNo != b.lastBlockFile {
		b.flushBlockFile(true)
		fileSize = 0
	}

	pos := NewFlatFilePos(fileNo, int64(fileSize))
	err := b.flatFileSeq.Allocate(pos, int64(blockSize))
	if err != nil {
		return nil, err
	}

	if fileNo != b.lastBlockFile {
		b.blockFiles = append(b.blockFiles, NewBlockFile(fileNo, false))
		b.lastBlockFile = fileNo
	}
	return pos, nil
}

// ReadRawBlock returns raw block bytes
func (b *BlockFileKeeper) ReadRawBlock(fileNo uint32, offset int64, blockSize int) ([]byte, error) {
	if fileNo > b.lastBlockFile {
		return nil, ErrFileOutOfRange
	}
	msgSize := BlockMessageHeaderLength + blockSize
	data, err := b.blockFiles[fileNo].ReadRawData(b.flatFileSeq, offset, msgSize)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(data[:MagicNoLength], MagicNo[:]) ||
		binary.LittleEndian.Uint64(data[MagicNoLength:BlockMessageHeaderLength]) != uint64(blockSize) {
		return nil, ErrReadBrokenData
	}
	return data[BlockMessageHeaderLength:], nil
}

// ReadRawTx returns raw transaction bytes
func (b *BlockFileKeeper) ReadRawTx(fileNo uint32, offsetBlock, offsetTxInBlock int64, txSize int) ([]byte, error) {
	if fileNo > b.lastBlockFile {
		return nil, ErrFileOutOfRange
	}

	// | ------ block message header ---- | ---------- raw block ---------- |
	// |    magic no    |  block size     |    tx0    |    tx1    |   ...   |
	targetOffset := offsetBlock + int64(BlockMessageHeaderLength) + offsetTxInBlock
	return b.blockFiles[fileNo].ReadRawData(b.flatFileSeq, targetOffset, txSize)
}
