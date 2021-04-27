package blockchain

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Sukhavati-Labs/go-miner/database/ldb"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/txscript"
	"github.com/Sukhavati-Labs/go-miner/wire"
	"github.com/stretchr/testify/assert"
)

type blockhexJson struct {
	BLKHEX string
}

type server struct {
	started bool
}

func (s *server) Stop() error {
	s.started = false
	return nil
}

type logger struct {
	lastLogHeight uint64
}

func (l *logger) LogBlockHeight(block *chainutil.Block) {
	l.lastLogHeight = block.MsgBlock().Header.Height
	fmt.Printf("log block height: %v, hash: %s\n", block.MsgBlock().Header.Height, block.Hash())
}

type chain struct {
	db database.DB
}

func (c *chain) GetBlockByHeight(height uint64) (*chainutil.Block, error) {
	sha, err := c.db.FetchBlockShaByHeight(height)
	if err != nil {
		return nil, err
	}
	block, err := c.db.FetchBlockBySha(sha)
	if err != nil {
		return nil, err
	}
	return block, nil
}

func (c *chain) NewestSha() (sha *wire.Hash, height uint64, err error) {
	sha, height1, err := c.db.NewestSha()
	if err != nil {
		return nil, 0, err
	}
	height = uint64(height1)
	return sha, height, nil
}

func decodeBlockFromString(blockhex string) (*chainutil.Block, error) {
	blockbuf, err := hex.DecodeString(blockhex)
	if err != nil {
		return nil, err
	}
	block, err := chainutil.NewBlockFromBytes(blockbuf, wire.Packet)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func preCommit(db database.DB, sha *wire.Hash) {
	chainDb := db.(*ldb.ChainDb)
	chainDb.Batch(0).Set(*sha)
	chainDb.Batch(0).Done()
	chainDb.Batch(1).Set(*sha)
	chainDb.Batch(1).Done()
}

func TestSubmitBlock(t *testing.T) {
	db, err := newTestChainDb()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	defer db.Close()

	blocks, err := loadTopNBlock(8)
	assert.Nil(t, err)

	for i := 1; i < 8; i++ {
		err = db.SubmitBlock(blocks[i])
		assert.Nil(t, err)

		blockHash := blocks[i].Hash()
		preCommit(db, blockHash)

		err = db.Commit(*blockHash)
		assert.Nil(t, err)

		_, height, err := db.NewestSha()
		assert.Nil(t, err)
		assert.Equal(t, blocks[i].Height(), height)
	}
}

func TestAddrIndexer(t *testing.T) {
	bc, teardown, err := newBlockChain()
	assert.Nil(t, err)
	defer teardown()

	adxr := bc.addrIndexer

	// block height is 3
	block, err := loadNthBlock(4)
	assert.Nil(t, err)

	// error will be returned for block 1 & block2 not connected
	err = adxr.SyncAttachBlock(block, nil)
	assert.Equal(t, errUnexpectedHeight, err)

	blocks, err := loadTopNBlock(22)
	assert.Nil(t, err)

	presentScriptHash := make(map[string]struct{})
	for i := 1; i < 21; i++ {
		block = blocks[i]
		isOrphan, err := bc.processBlock(block, BFNone)
		assert.Nil(t, err)
		assert.False(t, isOrphan)

		for _, tx := range block.Transactions() {
			for _, txout := range tx.MsgTx().TxOut {
				class, ops := txscript.GetScriptInfo(txout.PkScript)
				_, sh, _, err := txscript.GetParsedOpcode(ops, class)
				assert.Nil(t, err)
				presentScriptHash[wire.Hash(sh).String()] = struct{}{}
			}
		}
	}
	assert.Equal(t, uint64(20), bc.BestBlockHeight())
	t.Log("total present script hash in first 20 blocks:", presentScriptHash)

	// index block 21
	notPresentBefore21 := [][]byte{}
	cache := make(map[string]int)
	for i, tx := range blocks[21].Transactions() {
		for j, txout := range tx.MsgTx().TxOut {
			class, ops := txscript.GetScriptInfo(txout.PkScript)
			_, sh, _, err := txscript.GetParsedOpcode(ops, class)
			assert.Nil(t, err)
			if _, ok := presentScriptHash[wire.Hash(sh).String()]; !ok {
				if _, ok2 := cache[wire.Hash(sh).String()]; !ok2 {
					notPresentBefore21 = append(notPresentBefore21, sh[:])
					cache[wire.Hash(sh).String()] = i*10 + j
				}
			}
		}
	}
	t.Log("total only present in block 21:", cache)
	if len(notPresentBefore21) == 0 {
		t.Fatal("choose another block to continue test")
	}

	// before indexing block 21
	mp, err := bc.db.FetchScriptHashRelatedTx(notPresentBefore21, 0, 21)
	assert.Nil(t, err)
	assert.Zero(t, len(mp))

	node := NewBlockNode(&blocks[21].MsgBlock().Header, blocks[21].Hash(), BFNone)
	txStore, err := bc.fetchInputTransactions(node, blocks[21])
	assert.Nil(t, err)

	err = bc.db.SubmitBlock(blocks[21])
	assert.Nil(t, err)
	err = adxr.SyncAttachBlock(blocks[21], txStore)
	assert.Nil(t, err)
	blockHash := blocks[21].Hash()
	preCommit(bc.db, blockHash)
	err = bc.db.Commit(*blockHash)
	assert.Nil(t, err)

	// after indexing block 21
	mp, err = bc.db.FetchScriptHashRelatedTx(notPresentBefore21, 0, 22)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(mp))
	// assert.Equal(t, 3, len(mp[21]))
}

func TestGetAdxr(t *testing.T) {
	db, err := newTestChainDb()
	if err != nil {
		t.Errorf("%v", err)
		return
	}
	defer db.Close()

	// genesis
	block, err := loadNthBlock(1)
	assert.Nil(t, err)

	sha, height, err := db.FetchAddrIndexTip()
	assert.Nil(t, err)
	assert.Equal(t, uint64(0), height)
	assert.Equal(t, block.Hash(), sha)
}
