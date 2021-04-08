package blockchain

import (
	"encoding/binary"
	"fmt"
	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/txscript"
	"github.com/Sukhavati-Labs/go-miner/version"
	"github.com/Sukhavati-Labs/go-miner/wire"
	"sync"
)

type GovernAddressClass uint32

const (
	GovernUndefinedAddress GovernAddressClass = iota // 0
	GovernSupperAddress                              // 1
	GovernVersionAddress                             // 2
	GovernSenateAddress                              // 3
)

type GovernConfig interface {
	GetMeta() *GovernConfigMeta
	GetData() []byte
}

// GovernProposal
//  current                        future
//    \|/           shadow          \|/
// +--------+     +--------+     +--------+
// | config | --> | config | --->| config |
// +--------+     +--------+     +--------+
type GovernProposal struct {
	Id      GovernAddressClass
	current *GovernConfig
	future  *GovernConfig
}

type ChainGovern struct {
	sync.RWMutex
	db              database.DB
	server          Server
	proposalPool    map[GovernAddressClass]GovernProposal
	governAddresses map[wire.Hash]GovernAddressClass
}

func (g *ChainGovern) fetchGovernConfig(class GovernAddressClass, height uint64, includeShadow bool) ([]GovernConfig, error) {
	configs := make([]GovernConfig, 0)
	switch class {
	case GovernSenateAddress:
		{

			senates := make(database.SenateEquities, 0)
			senates = append(senates, database.SenateEquity{
				ScriptHash: [32]byte{250, 37, 244, 50, 232, 85, 83, 140, 181, 41, 129, 200, 157, 203, 88, 103, 6, 151, 63, 155, 4, 83, 70, 33, 11, 141, 37, 110, 185, 252, 183, 103},
				Weight:     1,
			})
			configs = append(configs, &GovernSenateConfig{
				meta: GovernConfigMeta{
					blockHeight:  0,
					activeHeight: 0,
					txId:         zeroHash,
					shadow:       false,
					id:           GovernSenateAddress,
				},
				senates: senates,
			})
		}
	case GovernVersionAddress:
		{
			configs = append(configs, &GovernVersionConfig{
				meta: GovernConfigMeta{
					blockHeight:  0,
					activeHeight: 0,
					txId:         zeroHash,
					shadow:       false,
					id:           GovernVersionAddress,
				},
				version: *version.GetVersion(),
			})
		}
	case GovernSupperAddress:
		{
			configs = append(configs, &GovernSupperConfig{
				meta: GovernConfigMeta{
					blockHeight:  0,
					activeHeight: 0,
					txId:         zeroHash,
					shadow:       false,
					id:           GovernSupperAddress,
				},
				addresses: make(map[string]uint32),
			})
		}
	default:
		return nil, fmt.Errorf("can't find config")
	}
	return configs, nil
}

// FetchEnabledGovernConfig
func (chain *Blockchain) FetchEnabledGovernConfig(class uint32) (*GovernConfig, error) {
	return chain.chainGovern.FetchEnabledGovernConfig(GovernAddressClass(class), chain.BestBlockHeight())
}

// FetchEnabledGovernConfig fetch next block height enable config
// Only one version is enabled at a time
func (g *ChainGovern) FetchEnabledGovernConfig(class GovernAddressClass, height uint64) (*GovernConfig, error) {
	g.Lock()
	defer g.Unlock()
	proposal, ok := g.proposalPool[class]
	if !ok {
		return nil, fmt.Errorf("can't find govern address class")
	}
	if proposal.current == nil {
		configs, err := g.fetchGovernConfig(class, height, false)
		if err != nil {
			return nil, err
		}
		l := len(configs)
		if l == 0 {
			return nil, fmt.Errorf("configs is empty! ")
		}
		if l == 1 {
			proposal.current = &configs[0]
		} else {
			proposal.current = &configs[0]
			proposal.future = &configs[l-1]
		}
	}
	if proposal.future == nil {
		return proposal.current, nil
	}
	if (*proposal.future).GetMeta().activeHeight >= height {
		proposal.current = proposal.future
		proposal.future = nil
	}
	if proposal.current != nil {
		return proposal.current, nil
	}
	return nil, fmt.Errorf("can't find config")
}

func (g *ChainGovern) SyncAttachBlock(block *chainutil.Block, txStore TxStore) error {
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
		for i, _ := range tx.TxOut() {
			info := tx.GetPkScriptInfo(i)
			scriptClass := txscript.ScriptClass(info.Class)
			if scriptClass != txscript.WitnessV0ScriptHashTy {
				break
			}
			curClass, ok := g.governAddresses[info.ScriptHash]
			if !ok {
				continue
			}
			if curClass == GovernUndefinedAddress {
				continue
			}
			addressClass = curClass
		}
		if addressClass == GovernUndefinedAddress {
			continue
		}
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
		err := g.UpdateConfig(addressClass, block.Height(), tx.Hash(), tx.MsgTx().Payload)
		if err != nil {
			return err
		}
	}
	return nil
}
func (g *ChainGovern) SyncDetachBlock(block *chainutil.Block) error {
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
		for i, _ := range tx.TxOut() {
			info := tx.GetPkScriptInfo(i)
			scriptClass := txscript.ScriptClass(info.Class)
			if scriptClass != txscript.WitnessV0ScriptHashTy {
				continue
			}
			curClass, ok := g.governAddresses[info.ScriptHash]
			if !ok {
				continue
			}
			if GovernUndefinedAddress == curClass {
				continue
			}
			addressClass = curClass
		}
		if GovernUndefinedAddress == addressClass {
			continue
		}
	}
	return nil
}
func (g *ChainGovern) UpdateConfig(class GovernAddressClass, height uint64, txSha *wire.Hash, payload []byte) error {
	g.Lock()
	defer g.Unlock()
	prop, ok := g.proposalPool[class]
	if !ok {
		return fmt.Errorf("govern can't find this class")
	}
	newConfig, err := DecodeGovernConfig(class, height, txSha, payload)
	if err != nil {
		return err
	}
	if prop.future != nil {
		//config := *prop.future
		//config.SetShadow()
		//g.db.InsertGovernConfig(uint32(config.GetId()),config.GetBlockHeight(),config.GetTxId(),config.GetData())
		prop.future = &newConfig
	}
	if (*prop.future).GetMeta().GetActiveHeight() >= height {
		prop.current = prop.future
		prop.future = nil
	}
	return nil
}

func NewChainGovern(db database.DB, server Server) (*ChainGovern, error) {
	cg := &ChainGovern{
		db:              db,
		server:          server,
		proposalPool:    make(map[GovernAddressClass]GovernProposal),
		governAddresses: make(map[wire.Hash]GovernAddressClass),
	}
	cg.proposalPool[GovernSupperAddress] = GovernProposal{Id: GovernSupperAddress}
	cg.proposalPool[GovernVersionAddress] = GovernProposal{Id: GovernVersionAddress}
	cg.proposalPool[GovernSenateAddress] = GovernProposal{Id: GovernSenateAddress}
	return cg, nil
}

type GovernConfigMeta struct {
	blockHeight  uint64
	activeHeight uint64
	shadow       bool
	txId         *wire.Hash
	id           GovernAddressClass
}

func (m *GovernConfigMeta) GetId() GovernAddressClass {
	return m.id
}

func (m *GovernConfigMeta) GetBlockHeight() uint64 {
	return m.blockHeight
}

func (m *GovernConfigMeta) GetActiveHeight() uint64 {
	return m.activeHeight
}

func (m *GovernConfigMeta) IsShadow() bool {
	return m.shadow
}

func (m *GovernConfigMeta) GetTxId() *wire.Hash {
	return m.txId
}
func (m *GovernConfigMeta) SetShadow() {
	m.shadow = true
}

type GovernSenateConfig struct {
	meta    GovernConfigMeta
	senates database.SenateEquities
}

func (gs *GovernSenateConfig) GetMeta() *GovernConfigMeta {
	return &gs.meta
}

func (gs *GovernSenateConfig) GetData() []byte {
	return nil
}

func (gs *GovernSenateConfig) GetNodes() database.SenateEquities {
	return gs.senates
}

type GovernVersionConfig struct {
	meta    GovernConfigMeta
	version version.Version
}

func (gv *GovernVersionConfig) GetMeta() *GovernConfigMeta {
	return &gv.meta
}

func (gv *GovernVersionConfig) GetData() []byte {
	return nil
}

func (gv *GovernVersionConfig) GetVersion() *version.Version {
	return &gv.version
}

type GovernSupperConfig struct {
	meta      GovernConfigMeta
	addresses map[string]uint32
}

func (gsc *GovernSupperConfig) GetMeta() *GovernConfigMeta {
	return &gsc.meta
}

func (gsc *GovernSupperConfig) GetData() []byte {
	return nil
}

func (gsc *GovernSupperConfig) GetAddresses() map[string]uint32 {
	return gsc.addresses
}

func DecodeGovernConfig(class GovernAddressClass, blockHeight uint64, txSha *wire.Hash, data []byte) (GovernConfig, error) {
	if len(data) <= 9 {
		return nil, fmt.Errorf("error data length")
	}
	activeHeight := binary.LittleEndian.Uint64(data[1:9])
	shadow := data[0] == 0x0
	switch class {
	case GovernSenateAddress:
		return &GovernSenateConfig{
			meta: GovernConfigMeta{
				blockHeight:  blockHeight,
				activeHeight: activeHeight,
				shadow:       shadow,
			},
		}, nil
	default:
		{
			return nil, fmt.Errorf("unsupported config class")
		}
	}
}
