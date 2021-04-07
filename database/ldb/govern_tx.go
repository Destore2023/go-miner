package ldb

import (
	"encoding/binary"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

//

var (
	recordGovernTx    = []byte("TXG")
	recordGovernTxLen = len(recordGovernTx)
)

const (
	//
	//  +---------+--------+--------+-------------+
	//  | Prefix  | id     |height  |  txId       |
	//  |---------+--------+--------+-------------+
	//  | 3 bytes |4 bytes |8 bytes | 32 bytes    |
	//  +---------+--------+--------+-------------+
	//           \|/
	//  +---------+---------------+---------------------+
	//  | shadow  | active height | data n bytes        |
	//  +---------+---------------+---------------------+
	//  | 1 byte  | 8 bytes       | n bytes             |
	//  +---------+---------------+---------------------+
	governanceKeyLength = 47
)

type governConfig struct {
	id           uint32     // 4 bytes
	blockHeight  uint64     // 8 bytes
	txSha        *wire.Hash // 32 bytes
	shadow       bool       // 1 byte  0 enable | 1 shadow
	activeHeight uint64     // 8 bytes
	data         []byte     // var
	delete       bool       // 1 bytes
}

type governConfigMapKey struct {
	id          uint32     // 4  bytes
	blockHeight uint64     // 8 bytes
	txSha       *wire.Hash // 32 bytes
}

func makeGovernConfigMapKeyToKey(mapKey governConfigMapKey) []byte {
	key := make([]byte, governanceKeyLength)
	copy(key[0:recordGovernTxLen], recordGovernTx)
	binary.LittleEndian.PutUint32(key[recordGovernTxLen:recordGovernTxLen+4], mapKey.id)
	binary.LittleEndian.PutUint64(key[recordGovernTxLen+4:recordGovernTxLen+12], mapKey.blockHeight)
	copy(key[recordGovernTxLen+12:governanceKeyLength], mapKey.txSha[:])
	return key
}

func (db *ChainDb) InsertGovernConfig(id uint32, height, activeHeight uint64, txSha *wire.Hash, data []byte) error {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()
	return db.insertGovernConfig(id, height, activeHeight, txSha, data)
}

func (db *ChainDb) fetchGovernConfig(class uint32, height uint64, includeShadow bool) error {
	panic("implement me")
}

func (db *ChainDb) insertGovernConfig(id uint32, height, activeHeight uint64, txSha *wire.Hash, data []byte) error {
	key := governConfigMapKey{
		id:          id,
		blockHeight: height,
		txSha:       txSha,
	}
	db.governConfigMap[key] = &governConfig{
		id:           id,
		blockHeight:  height,
		activeHeight: activeHeight,
		txSha:        txSha,
		data:         data,
		delete:       false,
	}
	return nil
}

// FetchGovernConfig only Fetch enabled config
func (db *ChainDb) FetchGovernanceConfig(id uint32, includeShadow bool) (*governConfig, error) {

	//return stakingAwardedRecords, nil
	//switch id {
	//case database.GovernSenate:
	//	equities := make(database.SenateEquities, 0)
	//	equity := database.SenateEquity{Equity: 10000}
	//	copy(equity.ScriptHash[:], []byte{66, 57, 85, 6, 53, 140, 105, 183, 15, 139, 97, 206, 191, 36, 23, 212, 3, 76, 131, 253, 114, 63, 179, 86, 158, 109, 230, 247, 122, 208, 84, 197})
	//	equities = append(equities, equity)
	//	return database.GovernSenateNodesConfig{SenateEquities: equities}, nil
	//default:
	//	return nil, nil
	//}
	return nil, nil
}
