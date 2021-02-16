package ldb_test

import (
	"testing"

	"github.com/Sukhavati-Labs/go-miner/database/ldb"
	"github.com/stretchr/testify/assert"
)

func TestChainDb_InitByGenesisBlock(t *testing.T) {
	db, tearDown, err := GetDb("DbTest") // already InitByGenesisBlock
	assert.Nil(t, err)
	defer tearDown()

	genesis := blocks200[0]
	blockSha, height, err := db.NewestSha()
	assert.Nil(t, err)
	assert.Equal(t, uint64(0), height)
	assert.Equal(t, blockSha, genesis.Hash())
}

func TestChainDb_DeleteBlock(t *testing.T) {
	db, tearDown, err := GetDb("DbTest")
	assert.Nil(t, err)
	defer tearDown()

	err = initBlocks(db, 200)
	assert.Nil(t, err)
	_, height, err := db.NewestSha()
	assert.Nil(t, err)
	assert.Equal(t, uint64(199), height)

	for j := 199; j > 100; j-- {
		block := blocks200[j]
		err := db.DeleteBlock(block.Hash())
		assert.Nil(t, err)
		db.(*ldb.ChainDb).Batch(1).Set(*block.Hash())
		db.(*ldb.ChainDb).Batch(1).Done()
		err = db.Commit(*block.Hash())
	}

	newestHash, newestHeight, err := db.NewestSha()
	assert.Nil(t, err)
	assert.Equal(t, blocks200[100].Height(), newestHeight)
	assert.Equal(t, blocks200[100].Hash().String(), newestHash.String())
}

func TestChainDb_FetchBlockShaByHeight(t *testing.T) {
	db, tearDown, err := GetDb("DbTest")
	assert.Nil(t, err)
	defer tearDown()

	err = initBlocks(db, 100)
	assert.Nil(t, err)

	for j := 0; j < 100; j++ {
		blockSha, err := db.FetchBlockShaByHeight(uint64(j))
		assert.Nil(t, err)
		assert.Equal(t, blocks200[j].Hash().String(), blockSha.String())
	}
}

func TestChainDb_FetchBlockBySha(t *testing.T) {
	db, tearDown, err := GetDb("DbTest")
	assert.Nil(t, err)
	defer tearDown()

	err = initBlocks(db, 100)
	assert.Nil(t, err)

	for i, block := range blocks200[1:100] {
		localBlock, err := db.FetchBlockBySha(block.Hash())
		assert.Nil(t, err)
		assert.Equal(t, uint64(i+1), localBlock.Height())
	}
}

func TestChainDb_FetchBlockHeightBySha(t *testing.T) {
	db, tearDown, err := GetDb("DbTest")
	assert.Nil(t, err)
	defer tearDown()

	err = initBlocks(db, 100)
	assert.Nil(t, err)

	for _, block := range blocks200[1:100] {
		blockHeight, err := db.FetchBlockHeightBySha(block.Hash())
		assert.Nil(t, err)
		assert.Equal(t, blockHeight, block.Height())
	}
}

func TestChainDb_FetchBlockHeaderBySha(t *testing.T) {
	db, tearDown, err := GetDb("DbTest")
	assert.Nil(t, err)
	defer tearDown()

	err = initBlocks(db, 100)
	assert.Nil(t, err)

	for _, block := range blocks200[1:100] {
		header, err := db.FetchBlockHeaderBySha(block.Hash())
		assert.Nil(t, err)
		assert.Equal(t, header.Height, block.Height())
	}
}

func TestChainDb_FetchHeightRange(t *testing.T) {
	db, tearDown, err := GetDb("DbTest")
	assert.Nil(t, err)
	defer tearDown()

	err = initBlocks(db, 100)
	assert.Nil(t, err)

	hashs, err := db.FetchHeightRange(1, 100)
	assert.Nil(t, err)
	for i, block := range blocks200[1:100] {
		assert.Equal(t, block.Hash(), &hashs[i])
	}
}

func TestChainDb_ExistsSha(t *testing.T) {
	db, tearDown, err := GetDb("DbTest")
	assert.Nil(t, err)
	defer tearDown()

	err = initBlocks(db, 100)
	assert.Nil(t, err)

	for _, block := range blocks200[1:100] {
		exist, err := db.ExistsBlockSha(block.Hash())
		assert.Nil(t, err)
		assert.True(t, exist)
	}
}
