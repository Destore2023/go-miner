package blockchain

import (
	"bytes"
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
	GetData() ([]byte, error)
	SetBytes(data []byte) error
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
				addresses: make(map[wire.Hash]uint32),
			})
		}
	default:
		return nil, fmt.Errorf("can't find config")
	}
	return configs, nil
}

func (chain *Blockchain) FetchGovernConfig(class uint32, includeShadow bool) ([]*GovernConfig, error) {
	return nil, nil
}

// FetchEnabledGovernConfig fetch current enable config
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

// isGovernTransaction return GovernAddressClass height txSha payload
func (g *ChainGovern) isGovernTransaction(tx *chainutil.Tx, txStore TxStore) (GovernAddressClass, bool) {
	if tx == nil {
		return GovernUndefinedAddress, false
	}
	payload := tx.MsgTx().Payload
	if len(payload) == 0 {
		return GovernUndefinedAddress, false
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
		return GovernUndefinedAddress, false
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
		} else if addressClass == class {
			return addressClass, true
		}
	}
	return GovernUndefinedAddress, false
}

func (g *ChainGovern) CheckTransactionGovernPayload(tx *chainutil.Tx, txStore TxStore) error {
	addressClass, ok := g.isGovernTransaction(tx, txStore)
	if !ok {
		return nil
	}
	_, err := DecodeGovernConfig(addressClass, 0, tx.Hash(), tx.MsgTx().Payload)
	if err != nil {
		return err
	}
	return nil
}

func (g *ChainGovern) SyncAttachBlock(block *chainutil.Block, txStore TxStore) error {
	g.Lock()
	defer g.Unlock()
	transactions := block.Transactions()
	for _, tx := range transactions {
		if IsCoinBaseTx(tx.MsgTx()) {
			continue
		}
		addressClass, ok := g.isGovernTransaction(tx, txStore)
		if !ok {
			continue
		}
		err := g.updateConfig(addressClass, block.Height(), tx.Hash(), tx.MsgTx().Payload)
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
		//addressClass, ok := g.isGovernTransaction(tx, txStore)
		//rawTx, _, _, _, err := g.db.fetchTxDataBySha(tx.Hash())
		//if !ok {
		//	continue
		//}
		//err := g.updateConfig(addressClass, block.Height(), tx.Hash(), tx.MsgTx().Payload)
		//if err != nil {
		//	return err
		//}
	}
	return nil
}
func (g *ChainGovern) updateConfig(class GovernAddressClass, height uint64, txSha *wire.Hash, payload []byte) error {
	prop, ok := g.proposalPool[class]
	if !ok {
		return fmt.Errorf("govern can't find this class")
	}
	newConfig, err := DecodeGovernConfig(class, height, txSha, payload)
	if err != nil {
		return err
	}
	if prop.future != nil {
		config := *prop.future
		id := uint32(config.GetMeta().GetId())
		blockHeight := config.GetMeta().GetBlockHeight()
		activeHeight := config.GetMeta().GetActiveHeight()
		txId := config.GetMeta().GetTxId()
		data, err := config.GetData()
		if err != nil {
			return err
		}
		err = g.db.InsertGovernConfig(id, blockHeight, activeHeight, true, txId, data)
		if err != nil {
			return err
		}
		prop.future = &newConfig
	}
	id := uint32(newConfig.GetMeta().GetId())
	blockHeight := newConfig.GetMeta().GetBlockHeight()
	activeHeight := newConfig.GetMeta().GetActiveHeight()
	txId := newConfig.GetMeta().GetTxId()
	data, err := newConfig.GetData()
	if err != nil {
		return err
	}
	err = g.db.InsertGovernConfig(id, blockHeight, activeHeight, false, txId, data)
	if err != nil {
		return err
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

func (m *GovernConfigMeta) GetMetaBytes() []byte {
	key := make([]byte, 9)
	if m.shadow {
		key[0] = 0x01
	} else {
		key[0] = 0x00
	}
	binary.LittleEndian.PutUint64(key[1:9], m.activeHeight)
	return key
}

func (m *GovernConfigMeta) SetMetaBytes(header []byte) error {
	if len(header) < 9 {
		return fmt.Errorf("SetMetaBytes Invalid data length ")
	}
	if header[0] == 0x00 {
		m.shadow = false
	} else {
		m.shadow = true
	}
	m.activeHeight = binary.LittleEndian.Uint64(header[1:9])
	return nil
}

type GovernSenateConfig struct {
	meta    GovernConfigMeta
	senates database.SenateEquities
}

func (gs *GovernSenateConfig) GetMeta() *GovernConfigMeta {
	return &gs.meta
}

func (gs *GovernSenateConfig) GetData() ([]byte, error) {
	header := gs.meta.GetMetaBytes()
	buffer := bytes.NewBuffer(header)
	for _, eq := range gs.senates {
		value := make([]byte, 40)
		binary.LittleEndian.PutUint64(value[0:8], eq.Weight)
		copy(value[9:40], eq.ScriptHash[:])
		_, err := buffer.Write(value)
		if err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}

func (gs *GovernSenateConfig) SetBytes(data []byte) error {
	l := len(data)
	n := (l - 9) / 40
	if l < 9 || (l-9)%40 != 0 {
		return fmt.Errorf("GovernSenateConfig SetBytes Invalid data length ")
	}
	shadow := data[0] == 0x00
	activeHeight := binary.LittleEndian.Uint64(data[1:9])
	gs.meta.shadow = shadow
	gs.meta.activeHeight = activeHeight
	for i := 0; i < n; i++ {
		start := i * 40
		weight := binary.LittleEndian.Uint64(data[start+9 : start+17])
		scriptHash := data[start+17 : start+49]
		equity := database.SenateEquity{Weight: weight}
		copy(equity.ScriptHash[:], scriptHash)
		gs.senates = append(gs.senates, equity)
	}
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

func (gv *GovernVersionConfig) GetData() ([]byte, error) {
	header := gv.meta.GetMetaBytes()
	buffer := bytes.NewBuffer(header)
	value := make([]byte, 4)
	binary.LittleEndian.PutUint32(value, gv.version.GetMajorVersion())
	buffer.Write(value)
	binary.LittleEndian.PutUint32(value, gv.version.GetMinorVersion())
	buffer.Write(value)
	binary.LittleEndian.PutUint32(value, gv.version.GetPatchVersion())
	buffer.Write(value)
	return buffer.Bytes(), nil
}

func (gv *GovernVersionConfig) GetVersion() *version.Version {
	return &gv.version
}

func (gv *GovernVersionConfig) SetBytes(data []byte) error {
	l := len(data)
	if l != 21 {
		return fmt.Errorf("GovernVersionConfig SetBytes Invalid data length ")
	}
	shadow := data[0] == 0x00
	activeHeight := binary.LittleEndian.Uint64(data[1:9])
	gv.meta.shadow = shadow
	gv.meta.activeHeight = activeHeight
	majorVersion := binary.LittleEndian.Uint32(data[9:13])
	minorVersion := binary.LittleEndian.Uint32(data[13:17])
	patchVersion := binary.LittleEndian.Uint32(data[17:21])
	newVersion := version.NewVersion(majorVersion, minorVersion, patchVersion)
	gv.version = *newVersion
	return nil
}

type GovernSupperConfig struct {
	meta      GovernConfigMeta
	addresses map[wire.Hash]uint32
}

func (gsc *GovernSupperConfig) GetMeta() *GovernConfigMeta {
	return &gsc.meta
}

func (gsc *GovernSupperConfig) GetData() ([]byte, error) {
	header := gsc.meta.GetMetaBytes()
	buffer := bytes.NewBuffer(header)
	for hash, id := range gsc.addresses {
		value := make([]byte, 36)
		copy(value[0:32], hash[:])
		binary.LittleEndian.PutUint32(value[32:36], id)
		buffer.Write(value)
	}
	return buffer.Bytes(), nil
}

func (gsc *GovernSupperConfig) GetAddresses() map[wire.Hash]uint32 {
	return gsc.addresses
}

func (gsc *GovernSupperConfig) SetBytes(data []byte) error {
	l := len(data)
	n := (l - 9) / 36
	if l < 9 || (l-9)%36 != 0 {
		return fmt.Errorf("GovernSupperConfig SetBytes Invalid data length ")
	}
	shadow := data[0] == 0x00
	activeHeight := binary.LittleEndian.Uint64(data[1:9])
	gsc.meta.shadow = shadow
	gsc.meta.activeHeight = activeHeight
	for i := 0; i < n; i++ {
		start := i * 36
		newHash, err := wire.NewHash(data[start : start+32])
		if err != nil {
			return err
		}
		id := binary.LittleEndian.Uint32(data[start+32 : start+36])
		gsc.addresses[*newHash] = id
	}
	return nil
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
				txId:         txSha,
				id:           GovernSenateAddress,
			},
		}, nil
	case GovernVersionAddress:
		return &GovernVersionConfig{
			meta: GovernConfigMeta{
				blockHeight:  blockHeight,
				activeHeight: activeHeight,
				shadow:       shadow,
				txId:         txSha,
				id:           GovernVersionAddress,
			},
		}, nil
	case GovernSupperAddress:
		return &GovernSupperConfig{
			meta: GovernConfigMeta{
				blockHeight:  blockHeight,
				activeHeight: activeHeight,
				shadow:       shadow,
				txId:         txSha,
				id:           GovernSupperAddress,
			},
		}, nil
	default:
		{
			return nil, fmt.Errorf("unsupported config class")
		}
	}
}
