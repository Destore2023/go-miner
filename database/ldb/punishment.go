package ldb

import (
	"bytes"
	"encoding/binary"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/database/storage"
	"github.com/Sukhavati-Labs/go-miner/pocec"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

var (
	punishmentPrefix = []byte("PUNISH")
)

func punishmentPubKeyToKey(pk *pocec.PublicKey) []byte {
	keyBytes := pk.SerializeCompressed()
	key := make([]byte, len(keyBytes)+len(punishmentPrefix))
	copy(key, punishmentPrefix)
	copy(key[len(punishmentPrefix):], keyBytes[:])
	return key
}

func insertPunishmentAtomic(batch storage.Batch, fpk *wire.FaultPubKey) error {
	key := punishmentPubKeyToKey(fpk.PubKey)
	data, err := fpk.Bytes(wire.DB)
	if err != nil {
		return err
	}
	return batch.Put(key, data)
}

func insertPunishments(batch storage.Batch, fpks []*wire.FaultPubKey) error {
	for _, fpk := range fpks {
		err := insertPunishmentAtomic(batch, fpk)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *ChainDb) InsertPunishment(fpk *wire.FaultPubKey) error {

	return db.insertPunishment(fpk)
}

func (db *ChainDb) insertPunishment(fpk *wire.FaultPubKey) error {
	key := punishmentPubKeyToKey(fpk.PubKey)
	data, err := fpk.Bytes(wire.DB)
	if err != nil {
		return err
	}
	return db.localStorage.Put(key, data)
}

func dropPunishments(batch storage.Batch, pks []*wire.FaultPubKey) error {
	for _, pk := range pks {
		key := punishmentPubKeyToKey(pk.PubKey)
		batch.Delete(key)
	}
	return nil
}

func (db *ChainDb) ExistsPunishment(pk *pocec.PublicKey) (bool, error) {
	return db.existsPunishment(pk)
}

func (db *ChainDb) existsPunishment(pk *pocec.PublicKey) (bool, error) {
	key := punishmentPubKeyToKey(pk)
	return db.localStorage.Has(key)
}

func (db *ChainDb) FetchAllPunishment() ([]*wire.FaultPubKey, error) {

	return db.fetchAllPunishment()
}

func (db *ChainDb) fetchAllPunishment() ([]*wire.FaultPubKey, error) {
	res := make([]*wire.FaultPubKey, 0)
	iter := db.localStorage.NewIterator(storage.BytesPrefix(punishmentPrefix))
	defer iter.Release()

	for iter.Next() {
		fpk, err := wire.NewFaultPubKeyFromBytes(iter.Value(), wire.DB)
		if err != nil {
			return nil, err
		}
		res = append(res, fpk)
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}
	return res, nil
}

func insertBlockPunishments(batch storage.Batch, block *chainutil.Block) error {
	faultPks := block.MsgBlock().Proposals.PunishmentArea
	var b2 [2]byte
	binary.LittleEndian.PutUint16(b2[0:2], uint16(len(faultPks)))

	var shaListData bytes.Buffer
	shaListData.Write(b2[:])

	for _, fpk := range faultPks {
		sha := wire.DoubleHashH(fpk.PubKey.SerializeUncompressed())
		shaListData.Write(sha.Bytes())
		err := insertFaultPk(batch, block.Height(), fpk, &sha)
		if err != nil {
			return err
		}

		// table - PUNISH
		key := punishmentPubKeyToKey(fpk.PubKey)
		batch.Delete(key)
	}

	// table - BANHGT
	if len(faultPks) > 0 {
		heightIndex := faultPubKeyHeightToKey(block.Height())
		if err := batch.Put(heightIndex, shaListData.Bytes()); err != nil {
			return err
		}
	}
	return nil
}
