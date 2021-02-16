package blockchain

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/database/ldb"
	"github.com/Sukhavati-Labs/go-miner/database/storage"
	_ "github.com/Sukhavati-Labs/go-miner/database/storage/ldbstorage"
	"github.com/Sukhavati-Labs/go-miner/wire"
	"github.com/stretchr/testify/assert"
)

func loadBlocks(path string) []*chainutil.Block {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	blocks := make([]*chainutil.Block, 0, 50)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		buf, err := hex.DecodeString(scanner.Text())
		if err != nil {
			panic(err)
		}
		block, err := chainutil.NewBlockFromBytes(buf, wire.Packet)
		if err != nil {
			panic(err)
		}
		blocks = append(blocks, block)
	}
	return blocks
}

func newReorgTestChain(genesis *chainutil.Block, name string) (*Blockchain, func()) {
	path := fmt.Sprintf("./reorgtest/%s/blocks.db", name)
	cachePath := fmt.Sprintf("./reorgtest/%s/blocks.dat", name)
	fi, err := os.Stat(cachePath)
	if os.IsNotExist(err) {
		os.MkdirAll(cachePath, 0700)
	}
	if fi != nil {
		os.RemoveAll(cachePath)
		os.MkdirAll(cachePath, 0700)
	}
	fi, err = os.Stat(path)
	if os.IsNotExist(err) {
		os.MkdirAll(path, 0700)
	}
	if fi != nil {
		os.RemoveAll(path)
		os.MkdirAll(path, 0700)
	}

	stor, err := storage.CreateStorage("leveldb", path, nil)
	if err != nil {
		panic(err)
	}
	db, err := ldb.NewChainDb(path, stor)
	if err != nil {
		panic(err)
	}
	if err = db.InitByGenesisBlock(genesis); err != nil {
		panic(err)
	}
	bc, err := newTestBlockchain(db, cachePath)
	if err != nil {
		panic(err)
	}

	return bc, func() {
		db.Close()
		os.RemoveAll("./reorgtest/" + name)
	}
}

func TestForkBeforeStaking(t *testing.T) {
	// height: 0(main)... -> 30(main) -> 31(main) -> 31(fork) -> 32(fork) -> 33(fork) -> 32(main) -> 33(main) -> ...49(main)
	// i:      0      ...    30          31          32          33          34          35          36          ...52
	blocks := loadBlocks("./data/beforestaking.dat")
	assert.Equal(t, 53, len(blocks))
	assert.Equal(t, uint64(31), blocks[31].Height())
	assert.Equal(t, uint64(31), blocks[32].Height())
	assert.Equal(t, uint64(32), blocks[33].Height())
	assert.Equal(t, uint64(33), blocks[34].Height())
	assert.Equal(t, uint64(32), blocks[35].Height())

	copy(config.ChainParams.GenesisHash[:], blocks[0].Hash()[:])
	copy(config.ChainParams.GenesisBlock.Header.Challenge[:], blocks[0].MsgBlock().Header.Challenge[:])
	copy(config.ChainParams.GenesisBlock.Header.ChainID[:], blocks[0].MsgBlock().Header.ChainID[:])
	config.ChainParams.GenesisBlock.Header.Timestamp = blocks[0].MsgBlock().Header.Timestamp
	config.ChainParams.GenesisBlock.Header.Target = blocks[0].MsgBlock().Header.Target

	// Step 1. Just process blocks on main chain
	bc1, close1 := newReorgTestChain(blocks[0], "1")
	defer close1()

	for i := 1; i < 42; i++ {
		block := blocks[i]
		if i >= 32 && i <= 34 {
			continue
		}
		isOrphan, err := bc1.processBlock(block, BFNone)
		if err != nil {
			t.Fatal(err, i)
		}
		assert.False(t, isOrphan, i)
	}
	assert.Equal(t, uint64(38), bc1.BestBlockHeight())
	entries1 := bc1.db.ExportDbEntries()
	checkEntries(t, entries1)

	// Step 2. Process all blocks including forks
	bc2, close2 := newReorgTestChain(blocks[0], "2")
	defer close2()

	for i := 1; i < 42; i++ {
		block := blocks[i]
		isOrphan, err := bc2.processBlock(block, BFNone)
		assert.Nil(t, err)
		assert.False(t, isOrphan)
	}
	assert.Equal(t, uint64(38), bc2.BestBlockHeight())
	entries2 := bc2.db.ExportDbEntries()
	checkEntries(t, entries2)

	assert.Equal(t, bc1.BestBlockHash().String(), bc2.BestBlockHash().String())
	assert.True(t, len(entries1) <= len(entries2))

	sameKeyValue := 0
	diffFb := 0
	diffBlockHeight := 0
	gap := SumBlockFileSize(t, blocks[31], blocks[32], blocks[33], blocks[34])
	for k2, v2 := range entries2 {
		v1, ok := entries1[k2]
		if !ok {
			assert.Equal(t, "PUNISH", string(k2[:6]))
			continue
		}
		if bytes.Equal(v1, v2) {
			sameKeyValue++
			continue
		}
		if strings.HasPrefix(k2, "fb") {
			diffFb++
			continue
		}
		if !strings.HasPrefix(k2, "BLKHGT") {
			t.FailNow()
		}
		diffBlockHeight++
		CheckBLKHGT(t, []byte(k2), v1, v2, gap)
	}
	assert.Equal(t, len(entries1), sameKeyValue+diffFb+diffBlockHeight)
}

func TestForkBeforeStakingExpire(t *testing.T) {
	// height: 0(main)... -> 39(main) -> 40(main) ->40(fork) -> 41(fork) -> 42(fork) -> 41(main) -> 42(main) -> ...49(main)
	// i:      0      ...    39          40         41          42          43          44          45          ...52
	blocks := loadBlocks("./data/beforestakingexpire.dat")
	assert.Equal(t, 53, len(blocks))
	assert.Equal(t, uint64(40), blocks[40].Height()) // main
	assert.Equal(t, uint64(40), blocks[41].Height()) // fork
	assert.Equal(t, uint64(41), blocks[42].Height()) // fork
	assert.Equal(t, uint64(42), blocks[43].Height()) // fork
	assert.Equal(t, uint64(41), blocks[44].Height()) // main

	copy(config.ChainParams.GenesisHash[:], blocks[0].Hash()[:])
	copy(config.ChainParams.GenesisBlock.Header.Challenge[:], blocks[0].MsgBlock().Header.Challenge[:])
	copy(config.ChainParams.GenesisBlock.Header.ChainID[:], blocks[0].MsgBlock().Header.ChainID[:])
	config.ChainParams.GenesisBlock.Header.Timestamp = blocks[0].MsgBlock().Header.Timestamp
	config.ChainParams.GenesisBlock.Header.Target = blocks[0].MsgBlock().Header.Target

	// Step 1. Just process blocks on main chain
	bc1, close1 := newReorgTestChain(blocks[0], "1")
	defer close1()

	for i := 1; i < 47; i++ {
		block := blocks[i]
		if i >= 41 && i <= 43 {
			continue
		}
		isOrphan, err := bc1.processBlock(block, BFNone)
		if err != nil {
			t.Fatal(err, i)
		}
		assert.False(t, isOrphan, i)
	}
	assert.Equal(t, uint64(43), bc1.BestBlockHeight())
	entries1 := bc1.db.ExportDbEntries()
	checkEntries(t, entries1)

	// Step 2. Process all blocks including forks
	bc2, close2 := newReorgTestChain(blocks[0], "2")
	defer close2()

	for i := 1; i < 47; i++ {
		block := blocks[i]
		isOrphan, err := bc2.processBlock(block, BFNone)
		assert.Nil(t, err)
		assert.False(t, isOrphan)
	}
	assert.Equal(t, uint64(43), bc2.BestBlockHeight())
	entries2 := bc2.db.ExportDbEntries()
	checkEntries(t, entries2)

	assert.Equal(t, bc1.BestBlockHash().String(), bc2.BestBlockHash().String())
	assert.True(t, len(entries1) <= len(entries2))

	sameKeyValue := 0
	diffFb := 0
	diffBlockHeight := 0
	gap := SumBlockFileSize(t, blocks[40], blocks[41], blocks[42], blocks[43])
	for k2, v2 := range entries2 {
		v1, ok := entries1[k2]
		if !ok {
			assert.Equal(t, "PUNISH", string(k2[:6]))
			continue
		}
		if bytes.Equal(v1, v2) {
			sameKeyValue++
			continue
		}
		if strings.HasPrefix(k2, "fb") {
			diffFb++
			continue
		}
		if !strings.HasPrefix(k2, "BLKHGT") {
			t.FailNow()
		}
		diffBlockHeight++
		CheckBLKHGT(t, []byte(k2), v1, v2, gap)
	}
	assert.Equal(t, len(entries1), sameKeyValue+diffFb+diffBlockHeight)
}

func TestForkBeforeStakingReward(t *testing.T) {
	// height: 0(main)... -> 37(main) -> 38(main) -> 38(fork) -> 39(fork) -> 40(fork) -> 39(main) -> 40(main) -> ...49(main)
	// i:      0      ...    37          38          39          40          41          42          43          ...52
	blocks := loadBlocks("./data/beforestakingreward.dat")
	assert.Equal(t, 53, len(blocks))
	assert.Equal(t, uint64(38), blocks[38].Height()) // main
	assert.Equal(t, uint64(38), blocks[39].Height()) // fork
	assert.Equal(t, uint64(39), blocks[40].Height()) // fork
	assert.Equal(t, uint64(40), blocks[41].Height()) // fork
	assert.Equal(t, uint64(39), blocks[42].Height()) // main

	copy(config.ChainParams.GenesisHash[:], blocks[0].Hash()[:])
	copy(config.ChainParams.GenesisBlock.Header.Challenge[:], blocks[0].MsgBlock().Header.Challenge[:])
	copy(config.ChainParams.GenesisBlock.Header.ChainID[:], blocks[0].MsgBlock().Header.ChainID[:])
	config.ChainParams.GenesisBlock.Header.Timestamp = blocks[0].MsgBlock().Header.Timestamp
	config.ChainParams.GenesisBlock.Header.Target = blocks[0].MsgBlock().Header.Target

	// Step 1. Just process blocks on main chain
	bc1, close1 := newReorgTestChain(blocks[0], "1")
	defer close1()

	for i := 1; i < 45; i++ {
		block := blocks[i]
		if i >= 39 && i <= 41 {
			continue
		}
		isOrphan, err := bc1.processBlock(block, BFNone)
		if err != nil {
			t.Fatal(err, i)
		}
		assert.False(t, isOrphan, i)
	}
	assert.Equal(t, uint64(41), bc1.BestBlockHeight())
	entries1 := bc1.db.ExportDbEntries()
	checkEntries(t, entries1)

	// Step 2. Process all blocks including forks
	bc2, close2 := newReorgTestChain(blocks[0], "2")
	defer close2()

	for i := 1; i < 45; i++ {
		block := blocks[i]
		isOrphan, err := bc2.processBlock(block, BFNone)
		assert.Nil(t, err)
		assert.False(t, isOrphan)
	}
	assert.Equal(t, uint64(41), bc2.BestBlockHeight())
	entries2 := bc2.db.ExportDbEntries()
	checkEntries(t, entries2)

	assert.Equal(t, bc1.BestBlockHash().String(), bc2.BestBlockHash().String())
	assert.True(t, len(entries1) <= len(entries2))

	sameKeyValue := 0
	diffFb := 0
	diffBlockHeight := 0
	gap := SumBlockFileSize(t, blocks[38], blocks[39], blocks[40], blocks[41])
	for k2, v2 := range entries2 {
		v1, ok := entries1[k2]
		if !ok {
			assert.Equal(t, "PUNISH", string(k2[:6]))
			continue
		}
		if bytes.Equal(v1, v2) {
			sameKeyValue++
			continue
		}
		if strings.HasPrefix(k2, "fb") {
			diffFb++
			continue
		}
		if !strings.HasPrefix(k2, "BLKHGT") {
			t.FailNow()
		}
		diffBlockHeight++
		CheckBLKHGT(t, []byte(k2), v1, v2, gap)
	}
	assert.Equal(t, len(entries1), sameKeyValue+diffFb+diffBlockHeight)
}
func TestForkBeforeBinding(t *testing.T) {
	// height: 0(main)... -> 23(main) -> 24(main) -> 24(fork) -> 25(fork) -> 26(fork) -> 25(main) -> 26(main) -> ...49(main)
	// i:      0      ...    23          24          25          26          27          28          29          ...52
	blocks := loadBlocks("./data/beforebinding.dat")
	assert.Equal(t, 53, len(blocks))
	assert.Equal(t, uint64(24), blocks[24].Height()) // main
	assert.Equal(t, uint64(24), blocks[25].Height()) // fork
	assert.Equal(t, uint64(25), blocks[26].Height()) // fork
	assert.Equal(t, uint64(26), blocks[27].Height()) // fork
	assert.Equal(t, uint64(25), blocks[28].Height()) // main

	copy(config.ChainParams.GenesisHash[:], blocks[0].Hash()[:])
	copy(config.ChainParams.GenesisBlock.Header.Challenge[:], blocks[0].MsgBlock().Header.Challenge[:])
	copy(config.ChainParams.GenesisBlock.Header.ChainID[:], blocks[0].MsgBlock().Header.ChainID[:])
	config.ChainParams.GenesisBlock.Header.Timestamp = blocks[0].MsgBlock().Header.Timestamp
	config.ChainParams.GenesisBlock.Header.Target = blocks[0].MsgBlock().Header.Target

	// Step 1. Just process blocks on main chain
	bc1, close1 := newReorgTestChain(blocks[0], "1")
	defer close1()

	for i := 1; i < 32; i++ {
		block := blocks[i]
		if i >= 25 && i <= 27 {
			continue
		}
		isOrphan, err := bc1.processBlock(block, BFNone)
		if err != nil {
			t.Fatal(err, i)
		}
		assert.False(t, isOrphan, i)
	}
	assert.Equal(t, uint64(28), bc1.BestBlockHeight())
	entries1 := bc1.db.ExportDbEntries()
	checkEntries(t, entries1)

	// Step 2. Process all blocks including forks
	bc2, close2 := newReorgTestChain(blocks[0], "2")
	defer close2()

	for i := 1; i < 32; i++ {
		block := blocks[i]
		isOrphan, err := bc2.processBlock(block, BFNone)
		assert.Nil(t, err)
		assert.False(t, isOrphan)
	}
	assert.Equal(t, uint64(28), bc2.BestBlockHeight())
	entries2 := bc2.db.ExportDbEntries()
	checkEntries(t, entries2)

	assert.Equal(t, bc1.BestBlockHash().String(), bc2.BestBlockHash().String())
	assert.True(t, len(entries1) <= len(entries2))

	sameKeyValue := 0
	diffFb := 0
	diffBlockHeight := 0
	gap := SumBlockFileSize(t, blocks[24], blocks[25], blocks[26], blocks[27])
	for k2, v2 := range entries2 {
		v1, ok := entries1[k2]
		if !ok {
			assert.Equal(t, "PUNISH", string(k2[:6]))
			continue
		}
		if bytes.Equal(v1, v2) {
			sameKeyValue++
			continue
		}
		if strings.HasPrefix(k2, "fb") {
			diffFb++
			continue
		}
		if !strings.HasPrefix(k2, "BLKHGT") {
			t.FailNow()
		}
		diffBlockHeight++
		CheckBLKHGT(t, []byte(k2), v1, v2, gap)
	}
	assert.Equal(t, len(entries1), sameKeyValue+diffFb+diffBlockHeight)
}

func checkEntries(t *testing.T, entries map[string][]byte) {
	var (
		blockFileTotal   uint32
		blockFileCounter int
		maxBlockFileNo   uint32
	)
	for key, value := range entries {
		prefix := ""
		switch {
		case strings.HasPrefix(key, "ADDRINDEXVERSION"):
			prefix = "ADDRINDEXVERSION"
		case strings.HasPrefix(key, "BLKSHA"):
			prefix = "BLKSHA"
		case strings.HasPrefix(key, "BLKHGT"):
			prefix = "BLKHGT"
		case strings.HasPrefix(key, "BANPUB"):
			prefix = "BANPUB"
		case strings.HasPrefix(key, "BANHGT"):
			prefix = "BANHGT"
		case strings.HasPrefix(key, "DBSTORAGEMETA"):
			prefix = "DBSTORAGEMETA"
		case strings.HasPrefix(key, "MBP"):
			prefix = "MBP"
		case strings.HasPrefix(key, "PUNISH"):
			prefix = "PUNISH"
		case strings.HasPrefix(key, "TXD"):
			prefix = "TXD"
		case strings.HasPrefix(key, "TXS"):
			prefix = "TXS"
		case strings.HasPrefix(key, "TXL"):
			prefix = "TXL"
		case strings.HasPrefix(key, "TXU"):
			prefix = "TXU"
		case strings.HasPrefix(key, "HTS"):
			prefix = "HTS"
		case strings.HasPrefix(key, "HTGS"):
			prefix = "HTGS"
		case strings.HasPrefix(key, "STL"):
			prefix = "STL"
		case strings.HasPrefix(key, "STG"):
			prefix = "STG"
		case strings.HasPrefix(key, "STSG"):
			prefix = "STSG"
		case strings.HasPrefix(key, "fb"):
			if key == "fblatest" {
				blockFileTotal = binary.LittleEndian.Uint32(value) + 1
			} else {
				blockFileCounter++
				no := binary.LittleEndian.Uint32([]byte(key)[2:6])
				if no > maxBlockFileNo {
					maxBlockFileNo = no
				}
			}
			continue
		case strings.HasPrefix(key, "PUBKBL"):
			prefix = "PUBKBL"
		default:
			t.Fatal(key, value)
		}
		checkEntry(t, prefix, key, value)
	}
	assert.True(t, int(blockFileTotal) == blockFileCounter)
	assert.True(t, blockFileTotal == maxBlockFileNo+1)
}

func checkEntry(t *testing.T, prefix, key string, value []byte) {
	switch prefix {
	case "ADDRINDEXVERSION":
		assert.Equal(t, 16, len(key))
		assert.Equal(t, 2, len(value))
	case "BLKSHA":
		assert.Equal(t, 38, len(key))
		assert.Equal(t, 8, len(value))
	case "BLKHGT":
		assert.Equal(t, 14, len(key))
		assert.True(t, len(value) == 52)
	case "BANPUB":
		assert.Equal(t, 38, len(key))
		assert.True(t, len(value) > 41)
	case "BANHGT":
		assert.Equal(t, 14, len(key))
		count := binary.LittleEndian.Uint16(value[0:2])
		assert.Equal(t, int(2+count*64), len(value))
	case "DBSTORAGEMETA":
		assert.Equal(t, 13, len(key))
		assert.Equal(t, 40, len(value))
	case "MBP":
		assert.Equal(t, 44, len(key))
		assert.Equal(t, 0, len(value))
	case "PUNISH":
		assert.Equal(t, 39, len(key))
		assert.True(t, len(value) > 0)
	case "TXD":
		assert.Equal(t, 35, len(key))
		assert.True(t, len(value) > 16)
	case "TXS":
		assert.Equal(t, 35, len(key))
		assert.Equal(t, 20, len(value))
	case "TXL":
		assert.Equal(t, 55, len(key))
		assert.Equal(t, 40, len(value))
	case "TXU":
		assert.Equal(t, 55, len(key))
		assert.Equal(t, 40, len(value))
	case "HTS":
		assert.Equal(t, 43, len(key))
		assert.Equal(t, 4, len(value))
	case "HTGS":
		assert.Equal(t, 32, len(key))
		assert.Equal(t, 0, len(value))
	case "STL":
		assert.Equal(t, 43, len(key))
		bitmap := binary.LittleEndian.Uint32(value[0:4])
		assert.True(t, len(value) >= 15)
		shift := 0
		cur := 4
		for bitmap != 0 {
			if bitmap&0x01 != 0 {
				index := value[cur]
				assert.True(t, int(index) == shift)
				N := binary.LittleEndian.Uint16(value[cur+1 : cur+3])
				assert.NotZero(t, N)

				// check no duplication
				offsets := make(map[uint32]bool)
				for i := 0; i < int(N); i++ {
					offset := binary.LittleEndian.Uint32(value[cur+3+8*i:])
					_, ok := offsets[offset]
					assert.False(t, ok)
					offsets[offset] = true
				}

				cur += 3 + 8*int(N)
			}
			bitmap >>= 1
			shift++
		}
		assert.True(t, cur == len(value))
	case "STG":
		assert.Equal(t, 43, len(key))
		assert.Equal(t, 0, len(value))
	case "STSG":
		assert.Equal(t, 72, len(key))
		assert.Equal(t, 0, len(value))
	case "PUBKBL":
		assert.Equal(t, 9, len(value))
	default:
		t.Fatal(prefix, key, value)
	}
}

func CheckBLKHGT(t *testing.T, key, value1, value2 []byte, gapOffset uint64) {
	height := binary.LittleEndian.Uint64(key[6:])
	var sha1, sha2 wire.Hash
	copy(sha1[:], value1[0:32])
	copy(sha2[:], value2[0:32])
	// value1
	fileNo1 := binary.LittleEndian.Uint32(value1[32:])
	offset1 := binary.LittleEndian.Uint64(value1[36:])
	blockSize1 := binary.LittleEndian.Uint64(value1[44:])
	// value2(fork)
	fileNo2 := binary.LittleEndian.Uint32(value2[32:])
	offset2 := binary.LittleEndian.Uint64(value2[36:])
	blockSize2 := binary.LittleEndian.Uint64(value2[44:])

	assert.Equal(t, sha1, sha2)
	assert.Equal(t, fileNo1, fileNo2, height, sha1)
	assert.Equal(t, blockSize1, blockSize2, height, sha1)
	assert.Equal(t, gapOffset, offset2-offset1)
}

func SumBlockFileSize(t *testing.T, blocks ...*chainutil.Block) (sum uint64) {
	for _, block := range blocks {
		raw, err := block.Bytes(wire.DB)
		assert.Nil(t, err)
		sum += uint64(12 + len(raw))
	}
	return sum
}
