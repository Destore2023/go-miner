package ldb_test

import (
	"testing"

	"github.com/Sukhavati-Labs/go-miner/database/storage"
	"github.com/Sukhavati-Labs/go-miner/wire"
	"github.com/stretchr/testify/assert"
)

func TestLevelDb_FetchFaultPkBySha(t *testing.T) {
	db, tearDown, err := GetDb("DbTest")
	assert.Nil(t, err)
	defer tearDown()

	err = initBlocks(db, 4)
	assert.Nil(t, err)

	block4, err := loadNthBlock(4)
	assert.Nil(t, err)
	sha, err := db.FetchBlockShaByHeight(3)
	assert.Nil(t, err)
	assert.Equal(t, sha, block4.Hash())

	_, _, err = db.FetchFaultPubKeyBySha(sha)
	assert.Equal(t, storage.ErrNotFound, err)

	// insert fpk
	block5, err := loadNthBlock(5)
	assert.Nil(t, err)
	fpk := NewFaultPubKey()
	block5.MsgBlock().Proposals.PunishmentArea = []*wire.FaultPubKey{fpk}
	err = insertBlock(db, block5)
	assert.Nil(t, err)

	sha, err = db.FetchBlockShaByHeight(4)
	assert.Nil(t, err)
	assert.Equal(t, sha, block5.Hash())
	fpkSha := wire.DoubleHashH(fpk.PubKey.SerializeUncompressed())
	rFpk, h, err := db.FetchFaultPubKeyBySha(&fpkSha)
	assert.Nil(t, err)
	assert.Equal(t, 4, int(h))
	readBuf, err := rFpk.Bytes(wire.DB)
	assert.Nil(t, err)
	writeBuf, err := fpk.Bytes(wire.DB)
	assert.Nil(t, err)
	assert.Equal(t, readBuf, writeBuf)
}

func TestLevelDb_FetchAllFaultPks(t *testing.T) {
	db, tearDown, err := GetDb("DbTest")
	assert.Nil(t, err)
	defer tearDown()

	err = initBlocks(db, 4)
	assert.Nil(t, err)

	fpks, _, err := db.FetchAllFaultPubKeys()
	assert.Nil(t, err)
	assert.Zero(t, len(fpks))

	block5, err := loadNthBlock(5)
	assert.Nil(t, err)
	fpk := NewFaultPubKey()
	block5.MsgBlock().Proposals.PunishmentArea = []*wire.FaultPubKey{fpk}
	err = insertBlock(db, block5)
	assert.Nil(t, err)

	_, heights, err := db.FetchAllFaultPubKeys()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(heights))
	assert.Equal(t, uint64(4), heights[0])
}

func TestLevelDb_FetchFaultPkListByHeight(t *testing.T) {
	db, tearDown, err := GetDb("DbTest")
	assert.Nil(t, err)
	defer tearDown()

	err = initBlocks(db, 4)
	assert.Nil(t, err)

	block5, err := loadNthBlock(5)
	assert.Nil(t, err)
	fpk := NewFaultPubKey()
	block5.MsgBlock().Proposals.PunishmentArea = []*wire.FaultPubKey{fpk}
	err = insertBlock(db, block5)
	assert.Nil(t, err)

	fpks, err := db.FetchFaultPubKeyListByHeight(4)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(fpks))
	readBuf, err := fpks[0].Bytes(wire.DB)
	assert.Nil(t, err)
	writeBuf, err := fpk.Bytes(wire.DB)
	assert.Nil(t, err)
	assert.Equal(t, readBuf, writeBuf)
}

func TestLevelDb_ExistsFaultPk(t *testing.T) {
	db, tearDown, err := GetDb("DbTest")
	assert.Nil(t, err)
	defer tearDown()

	err = initBlocks(db, 4)
	assert.Nil(t, err)

	_, hgt, err := db.NewestSha()
	assert.Nil(t, err)
	assert.Equal(t, uint64(3), hgt)

	block5, err := loadNthBlock(5)
	assert.Nil(t, err)

	fpk := NewFaultPubKey()
	fpksha := wire.DoubleHashH(fpk.PubKey.SerializeUncompressed())

	exist, err := db.ExistsFaultPubKey(&fpksha)
	assert.Nil(t, err)
	assert.False(t, exist)

	block5.MsgBlock().Proposals.PunishmentArea = []*wire.FaultPubKey{fpk}
	err = insertBlock(db, block5)
	assert.Nil(t, err)

	_, hgt, err = db.NewestSha()
	assert.Nil(t, err)
	assert.Equal(t, uint64(4), hgt)

	exist, err = db.ExistsFaultPubKey(&fpksha)
	assert.Nil(t, err)
	assert.True(t, exist)
}
