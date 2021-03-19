package ldb

import (
	"encoding/binary"

	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/database/storage"

	"github.com/Sukhavati-Labs/go-miner/wire"
)

//

var (
	recordGovernanceTx = []byte("TXG")
)

const (
	//
	//  ---------------------------------------------------         -------------------
	//  | Prefix     |  type      |   txId                |         |    status       |
	//  |---------------------------------------------------------->-------------------
	//  |  3 bytes   |   4 bytes  |   32 bytes            |         |    1 byte       |
	//  --------------------------------------------------          -------------------
	governanceKeyLength = 39
)

type governanceConfigMapKey struct {
	configType int  // 4  bytes
	enabled    bool // 1  byte
}

func (db *ChainDb) insertGovernanceConfig(configType int, config *database.GovernanceConfig, enable bool) error {
	db.governanceConfigMap[configType] = config
	return nil
}

// FetchEnabledGovernanceConfig only Fetch enabled config
func (db *ChainDb) FetchEnabledGovernanceConfig(configType int) (database.GovernanceConfig, error) {

	//return stakingAwardedRecords, nil
	switch configType {
	case database.GovernanceSenate:
		equities := make(database.SenateEquities, 0)
		equity := database.SenateEquity{Equity: 10000}
		copy(equity.ScriptHash[:], []byte{66, 57, 85, 6, 53, 140, 105, 183, 15, 139, 97, 206, 191, 36, 23, 212, 3, 76, 131, 253, 114, 63, 179, 86, 158, 109, 230, 247, 122, 208, 84, 197})
		equities = append(equities, equity)
		return database.GovernanceSenateNodesConfig{SenateEquities: equities}, nil
	default:
		return nil, nil
	}
	return nil, nil
}

func (db *ChainDb) FetchGovernanceConfig(configType int, status byte) ([]database.GovernanceConfig, error) {
	governanceConfigs := make([]database.GovernanceConfig, 0)
	iter := db.localStorage.NewIterator(storage.BytesPrefix(recordGovernanceTx))
	defer iter.Release()
	for iter.Next() {
		key := iter.Key()
		if len(key) < 19 {
			continue
		}
		currentType := binary.LittleEndian.Uint32(key[len(recordGovernanceTx) : len(recordGovernanceTx)+4])
		if currentType != uint32(configType) {
			continue
		}
		value := iter.Value()
		if value[0] != status {
			continue
		}
		txId := new(wire.Hash)
		copy((*txId)[:], key[len(recordGovernanceTx)+4:governanceKeyLength])
		reply, err := db.FetchTxBySha(txId)
		if err != nil {
			return nil, err
		}
		if len(reply) == 0 {
			continue
		}

	}
	if err := iter.Error(); err != nil {
		return nil, err
	}

	return governanceConfigs, nil
}
