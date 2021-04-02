package blockchain

import (
	"encoding/json"
	"fmt"
	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/version"
	"github.com/Sukhavati-Labs/go-miner/wire"
	"math"
	"sync"
)

type GovernAddressClass uint32

const (
	GovernSupperAddress    GovernAddressClass = iota // 0
	GovernVersionAddress                             // 1
	GovernSenateAddress                              // 2
	GovernUndefinedAddress = GovernSupperAddress + math.MaxUint32
)

type GovernProposal interface {
	GetGovernAddressClass() GovernAddressClass
	UpgradeConfig(payload []byte) error
	GetHeight() uint64
	Bytes() []byte
	CallFunc() (bool, error)
}

// GovernVersionProposal
type GovernVersionProposal struct {
	id      uint32          `json:"id"`
	height  uint64          `json:"height"`
	version version.Version `json:"version"`
}

func (gv *GovernVersionProposal) GetGovernAddressClass() GovernAddressClass {
	return GovernVersionAddress
}
func (gv *GovernVersionProposal) UpgradeConfig(payload []byte) error {
	d := GovernVersionProposal{}
	err := json.Unmarshal(payload, d)
	if err != nil {
		return err
	}
	return nil
}
func (gv *GovernVersionProposal) GetHeight() uint64 {
	return gv.height
}

func (gv *GovernVersionProposal) Bytes() []byte {
	return nil
}

func (gv *GovernVersionProposal) CallFunc() (bool, error) {
	return true, nil
}

type GovernSenateProposal struct {
	height uint64
}

// GovernSenateAddress
func (gsv *GovernSenateProposal) GetGovernAddressClass() GovernAddressClass {
	return GovernSenateAddress
}

func (gsv *GovernSenateProposal) UpgradeConfig(payload []byte) error {
	return nil
}
func (gsv *GovernSenateProposal) GetHeight() uint64 {
	return gsv.height
}

func (gsv *GovernSenateProposal) Bytes() []byte {
	return nil
}

func (gsv *GovernSenateProposal) CallFunc() (bool, error) {
	return true, nil
}

type ChainGovern struct {
	sync.RWMutex
	db              database.DB
	server          Server
	proposalPool    map[GovernAddressClass]GovernProposal
	governAddresses map[wire.Hash]GovernAddressClass
}

func (g *ChainGovern) IsGovern(data []byte) bool {
	return true
}

func (g *ChainGovern) SyncGovernConfig(block *chainutil.Block, txStore TxStore) error {
	g.Lock()
	defer g.Unlock()
	transactions := block.Transactions()
	for _, tx := range transactions {
		if IsCoinBaseTx(tx.MsgTx()) {
			continue
		}
		payload := tx.MsgTx().Payload
		if len(payload) == 0 {
			continue
		}
		addressClass := GovernUndefinedAddress
		for _, txIn := range tx.TxIn() {
			preData, ok := txStore[txIn.PreviousOutPoint.Hash]
			if !ok {
				continue
			}
			publicKeyInfo := preData.Tx.GetPkScriptInfo(int(txIn.PreviousOutPoint.Index))
			class, ok := g.governAddresses[publicKeyInfo.ScriptHash]
			if !ok {
				break
			} else {
				addressClass = class
			}
		}
		if addressClass == GovernUndefinedAddress {
			continue
		}

		for i, _ := range tx.TxOut() {
			info := tx.GetPkScriptInfo(i)
			curClass, ok := g.governAddresses[info.ScriptHash]
			if !ok {
				continue
			}
			if curClass != addressClass {
				continue
			}
		}
		err := g.UpgradeConfig(addressClass, tx.MsgTx().Payload)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *ChainGovern) UpgradeConfig(class GovernAddressClass, payload []byte) error {
	prop, ok := g.proposalPool[class]
	if !ok {
		return fmt.Errorf("govern can't find this class")
	}
	return prop.UpgradeConfig(payload)
}

func NewChainGovern(db database.DB, server Server) (*ChainGovern, error) {
	cg := &ChainGovern{
		db:              db,
		server:          server,
		proposalPool:    make(map[GovernAddressClass]GovernProposal),
		governAddresses: make(map[wire.Hash]GovernAddressClass),
	}
	cg.proposalPool[GovernVersionAddress] = &GovernVersionProposal{}
	return cg, nil
}
