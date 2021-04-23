package blockchain

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/txscript"
	"github.com/Sukhavati-Labs/go-miner/version"
	"github.com/Sukhavati-Labs/go-miner/wire"
	"sync"
)

//  govern config
//  Govern Config Class
//  zero as undeclared identifier
const (
	// GovernSupperClass Update other govern address
	GovernSupperClass uint16 = 0x0001 // 1
	// GovernVersionClass Minimum version requirement
	GovernVersionClass uint16 = 0x0002 // 2
	// GovernSenateClass Update senate node information
	GovernSenateClass uint16 = 0x0003 // 3
)

const (
	GovernSupperConfigValueLen  = 34
	GovernVersionConfigValueLen = 4
	GovernSenateConfigValueLen  = 40
)

// UnsupportedGovernClassError describes an error where a govern config
// decoded has an unsupported class.
type UnsupportedGovernClassError uint16

func (e UnsupportedGovernClassError) Error() string {
	return fmt.Sprintf("unsupported govern class: %d", e)
}

// EmptyGovernConfigError
// The EmptyGovernConfigError error is thrown when the govern configuration cannot be found
type EmptyGovernConfigError uint16

func (e EmptyGovernConfigError) Error() string {
	return fmt.Sprintf("empty govern config! class :%d", e)
}

// InvalidGovernConfigFormatError Invalid data format
type InvalidGovernConfigFormatError uint16

func (e InvalidGovernConfigFormatError) Error() string {
	return fmt.Sprintf("Invalid Govern class: %d config data format ", e)
}

type GovernConfig interface {
	// GovernClass return govern class
	GovernClass() uint16
	// BlockHeight return block height
	BlockHeight() uint64
	// ActiveHeight return active height , active height >= block height
	ActiveHeight() uint64
	// IsShadow shadow
	IsShadow() bool
	// TxSha return tx sha
	TxSha() *wire.Hash
	// String return string format
	String() string
	// ConfigData return config in database format
	// include: only special config
	ConfigData() []byte
}

// GovernProposal
//  current                        future
//    \|/           shadow          \|/
// +--------+     +--------+     +--------+
// | config | --> | config | --->| config |
// +--------+     +--------+     +--------+
type GovernProposal struct {
	Id      uint16
	current GovernConfig
	future  GovernConfig
}

type ChainGovern struct {
	sync.RWMutex
	db              database.DB
	server          Server
	proposalPool    map[uint16]GovernProposal
	governAddresses map[wire.Hash]uint16
}

func (g *ChainGovern) fetchGovernConfig(class uint16, height uint64, includeShadow bool) ([]GovernConfig, error) {
	configs := make([]GovernConfig, 0)
	configDataList, err := g.db.FetchGovernConfigData(class, height, includeShadow)
	if err != nil {
		return nil, err
	}
	for _, configData := range configDataList {
		governConfig, err := DecodeGovernConfigFromData(configData)
		if err != nil {
			return nil, err
		}
		configs = append(configs, governConfig)
	}
	return configs, nil
}

func (chain *Blockchain) FetchGovernConfig(class uint32, includeShadow bool) ([]GovernConfig, error) {
	return nil, nil
}

// FetchEnabledGovernConfig fetch current enable config
func (chain *Blockchain) FetchEnabledGovernConfig(class uint16) (GovernConfig, error) {
	return chain.chainGovern.FetchEnabledGovernConfig(class, chain.BestBlockHeight())
}

// FetchEnabledGovernConfig fetch next block height enable config
// Only one version is enabled at a time
func (g *ChainGovern) FetchEnabledGovernConfig(class uint16, height uint64) (GovernConfig, error) {
	g.Lock()
	defer g.Unlock()
	proposal, ok := g.proposalPool[class]
	if !ok {
		return nil, UnsupportedGovernClassError(class)
	}
	if proposal.current == nil {
		configs, err := g.fetchGovernConfig(class, height, false)
		if err != nil {
			return nil, err
		}
		l := len(configs)
		if l == 0 {
			return nil, EmptyGovernConfigError(class)
		}
		if l == 1 {
			proposal.current = configs[0]
		} else {
			proposal.current = configs[0]
			proposal.future = configs[l-1]
		}
	}
	if proposal.future == nil {
		return proposal.current, nil
	}
	if proposal.future.ActiveHeight() >= height {
		proposal.current = proposal.future
		proposal.future = nil
	}
	if proposal.current != nil {
		return proposal.current, nil
	}
	return nil, EmptyGovernConfigError(class)
}

// isGovernTransaction return GovernAddressClass height txSha payload
func (g *ChainGovern) isGovernTransaction(tx *chainutil.Tx, txStore TxStore) (uint16, bool) {
	if tx == nil {
		return 0, false
	}
	payload := tx.MsgTx().Payload
	if len(payload) == 0 {
		return 0, false
	}
	var addressClass uint16
	for i, _ := range tx.TxOut() {
		info := tx.GetPkScriptInfo(i)
		scriptClass := txscript.ScriptClass(info.Class)
		// TODO only multi sign type
		if scriptClass != txscript.WitnessV0ScriptHashTy {
			return 0, false
		}
		curClass, ok := g.governAddresses[info.ScriptHash]
		if !ok {
			return 0, false
		}
		if curClass == 0 {
			return 0, false
		}
		addressClass = curClass
	}
	if addressClass == 0 {
		return 0, false
	}
	for _, txIn := range tx.TxIn() {
		preData, ok := txStore[txIn.PreviousOutPoint.Hash]
		if !ok {
			return 0, false
		}
		publicKeyInfo := preData.Tx.GetPkScriptInfo(int(txIn.PreviousOutPoint.Index))
		class, ok := g.governAddresses[publicKeyInfo.ScriptHash]
		if !ok {
			return 0, false
		}
		if addressClass == class {
			return addressClass, true
		}
	}
	return 0, false
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
func (g *ChainGovern) updateConfig(class uint16, height uint64, txSha *wire.Hash, payload []byte) error {
	prop, ok := g.proposalPool[class]
	if !ok {
		return UnsupportedGovernClassError(class)
	}
	newConfig, err := DecodeGovernConfig(class, height, txSha, payload)
	if err != nil {
		return err
	}
	if prop.future != nil {
		err = g.db.InsertGovernConfig(prop.current.GovernClass(), prop.current.BlockHeight(), prop.current.ActiveHeight(), true, prop.current.TxSha(), prop.current.ConfigData())
		if err != nil {
			return err
		}
	}
	prop.future = newConfig
	err = g.db.InsertGovernConfig(newConfig.GovernClass(), newConfig.BlockHeight(), newConfig.ActiveHeight(), false, newConfig.TxSha(), newConfig.ConfigData())
	if err != nil {
		return err
	}
	if height >= prop.future.ActiveHeight() {
		prop.current = prop.future
		prop.future = nil
	}
	return nil
}

func NewChainGovern(db database.DB, server Server) (*ChainGovern, error) {
	cg := &ChainGovern{
		db:              db,
		server:          server,
		proposalPool:    make(map[uint16]GovernProposal),
		governAddresses: make(map[wire.Hash]uint16),
	}
	cg.proposalPool[GovernSupperClass] = GovernProposal{Id: GovernSupperClass}
	cg.proposalPool[GovernVersionClass] = GovernProposal{Id: GovernVersionClass}
	cg.proposalPool[GovernSenateClass] = GovernProposal{Id: GovernSenateClass}
	return cg, nil
}

// Govern Config

// GovernSupperConfig Supper address
type GovernSupperConfig struct {
	blockHeight  uint64               `json:"block_height"`
	activeHeight uint64               `json:"active_height"`
	shadow       bool                 `json:"shadow"`
	txSha        *wire.Hash           `json:"tx_sha"`
	addresses    map[wire.Hash]uint16 `json:"addresses"`
}

func (gsc *GovernSupperConfig) GovernClass() uint16 {
	return GovernSupperClass
}

func (gsc *GovernSupperConfig) BlockHeight() uint64 {
	return gsc.blockHeight
}

func (gsc *GovernSupperConfig) ActiveHeight() uint64 {
	return gsc.activeHeight
}

func (gsc *GovernSupperConfig) IsShadow() bool {
	return gsc.shadow
}

func (gsc *GovernSupperConfig) TxSha() *wire.Hash {
	return gsc.txSha
}

func (gsc *GovernSupperConfig) String() string {
	return ""
}

func (gsc *GovernSupperConfig) ConfigData() []byte {
	var buffer bytes.Buffer
	// 32 hash --> 2 id
	for hash, id := range gsc.addresses {
		value := make([]byte, GovernSupperConfigValueLen)
		copy(value[0:32], hash[:])
		binary.LittleEndian.PutUint16(value[32:GovernSupperConfigValueLen], id)
		buffer.Write(value)
	}
	return buffer.Bytes()
}

func (gsc *GovernSupperConfig) GetAddresses() map[wire.Hash]uint16 {
	return gsc.addresses
}

// GovernVersionConfig Minimum version
type GovernVersionConfig struct {
	blockHeight  uint64           `json:"block_height"`
	activeHeight uint64           `json:"active_height"`
	shadow       bool             `json:"shadow"`
	txSha        *wire.Hash       `json:"tx_sha"`
	version      *version.Version `json:"version"`
}

func (gv *GovernVersionConfig) GovernClass() uint16 {
	return GovernVersionClass
}

func (gv *GovernVersionConfig) BlockHeight() uint64 {
	return gv.blockHeight
}

func (gv *GovernVersionConfig) ActiveHeight() uint64 {
	return gv.activeHeight
}

func (gv *GovernVersionConfig) IsShadow() bool {
	return gv.shadow
}

func (gv *GovernVersionConfig) TxSha() *wire.Hash {
	return gv.txSha
}

func (gv *GovernVersionConfig) String() string {
	return ""
}

func (gv *GovernVersionConfig) ConfigData() []byte {
	var buffer bytes.Buffer
	value := make([]byte, GovernVersionConfigValueLen)
	binary.LittleEndian.PutUint32(value, gv.version.GetMajorVersion())
	buffer.Write(value)
	binary.LittleEndian.PutUint32(value, gv.version.GetMinorVersion())
	buffer.Write(value)
	binary.LittleEndian.PutUint32(value, gv.version.GetPatchVersion())
	buffer.Write(value)
	return buffer.Bytes()
}

func (gv *GovernVersionConfig) GetVersion() *version.Version {
	return gv.version
}

// GovernSenateConfig Senate Node Information
type GovernSenateConfig struct {
	blockHeight  uint64                   `json:"block_height"`
	activeHeight uint64                   `json:"active_height"`
	shadow       bool                     `json:"shadow"`
	txSha        *wire.Hash               `json:"tx_sha"`
	senates      []*database.SenateWeight `json:"senates"`
}

func (gs *GovernSenateConfig) GovernClass() uint16 {
	return GovernSenateClass
}

func (gs *GovernSenateConfig) BlockHeight() uint64 {
	return gs.blockHeight
}

func (gs *GovernSenateConfig) ActiveHeight() uint64 {
	return gs.activeHeight
}

func (gs *GovernSenateConfig) IsShadow() bool {
	return gs.shadow
}

func (gs *GovernSenateConfig) TxSha() *wire.Hash {
	return gs.txSha
}

func (gs *GovernSenateConfig) String() string {
	return ""
}

func (gs *GovernSenateConfig) ConfigData() []byte {
	var buffer bytes.Buffer
	for _, senate := range gs.senates {
		value := make([]byte, GovernSenateConfigValueLen)
		copy(value[0:32], senate.ScriptHash[:])
		binary.LittleEndian.PutUint64(value[32:GovernSenateConfigValueLen], senate.Weight)
		buffer.Write(value)
	}
	return buffer.Bytes()
}

func (gs *GovernSenateConfig) GetNodes() []*database.SenateWeight {
	return gs.senates
}

func DecodeGovernConfigFromData(configData *database.GovernConfigData) (GovernConfig, error) {
	class := configData.Id
	blockHeight := configData.BlockHeight
	txSha := configData.TxSha
	activeHeight := configData.ActiveHeight
	shadow := configData.Shadow
	data := configData.Data
	l := len(data)
	switch class {
	case GovernSupperClass:
		{
			config := &GovernSupperConfig{
				blockHeight:  blockHeight,
				activeHeight: activeHeight,
				shadow:       shadow,
				txSha:        txSha,
				addresses:    make(map[wire.Hash]uint16),
			}
			if l%GovernSupperConfigValueLen != 0 {
				return nil, InvalidGovernConfigFormatError(class)
			}
			n := l / GovernSupperConfigValueLen
			for i := 0; i < n; i++ {
				start := i * GovernSupperConfigValueLen
				scriptHash := data[start : start+32]
				hash, err := wire.NewHash(scriptHash)
				if err != nil {
					return nil, err
				}
				id := binary.LittleEndian.Uint16(data[start+32 : start+GovernSupperConfigValueLen])
				config.addresses[*hash] = id
			}
			return config, nil
		}
	case GovernVersionClass:
		{
			config := &GovernVersionConfig{
				blockHeight:  blockHeight,
				activeHeight: activeHeight,
				shadow:       shadow,
				txSha:        txSha,
			}
			if l != 3*GovernVersionConfigValueLen {
				return nil, InvalidGovernConfigFormatError(class)
			}
			majorVersion := binary.LittleEndian.Uint32(data[0:4])
			minorVersion := binary.LittleEndian.Uint32(data[4:8])
			patchVersion := binary.LittleEndian.Uint32(data[8:12])
			config.version = version.NewVersion(majorVersion, minorVersion, patchVersion)
			return config, nil
		}
	case GovernSenateClass:
		{
			config := &GovernSenateConfig{
				blockHeight:  blockHeight,
				activeHeight: activeHeight,
				shadow:       shadow,
				txSha:        txSha,
				senates:      make([]*database.SenateWeight, 0),
			}
			if l%GovernSenateConfigValueLen != 0 {
				return nil, InvalidGovernConfigFormatError(class)
			}
			n := l / GovernSenateConfigValueLen
			for i := 0; i < n; i++ {
				start := i * GovernSenateConfigValueLen
				scriptHash := data[start : start+32]
				weight := binary.LittleEndian.Uint64(data[start+32 : start+GovernSenateConfigValueLen])
				senateWeight := database.SenateWeight{Weight: weight}
				copy(senateWeight.ScriptHash[:], scriptHash)
				config.senates = append(config.senates, &senateWeight)
			}
			return config, nil
		}
	default:
		return nil, UnsupportedGovernClassError(class)
	}
}

type GovernVersionJson struct {
	activeHeight uint64 `json:"active_height"`
	version      string `json:"version"`
}

type GovernSupperAddressJson struct {
	activeHeight uint64            `json:"active_height"`
	addresses    map[string]uint16 `json:"addresses"`
}

type GovernSenateJson struct {
	activeHeight uint64            `json:"active_height"`
	senates      map[string]uint64 `json:"senates"`
}

// DecodeGovernConfig Decode configuration information from the transaction's payload
func DecodeGovernConfig(class uint16, blockHeight uint64, txSha *wire.Hash, payload []byte) (GovernConfig, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("error class:%d govern payload data length ", class)
	}
	switch class {
	case GovernSenateClass:
		senateJson := GovernSenateJson{}
		err := json.Unmarshal(payload, senateJson)
		if err != nil {
			return nil, err
		}
		config := GovernSenateConfig{
			blockHeight:  blockHeight,
			activeHeight: senateJson.activeHeight,
			shadow:       false,
			txSha:        txSha,
			senates:      make([]*database.SenateWeight, 0),
		}
		for scriptHashString, weight := range senateJson.senates {
			scriptHashBytes, err := hex.DecodeString(scriptHashString)
			if err != nil {
				return nil, err
			}
			scriptHash, err := wire.NewHash(scriptHashBytes)
			if err != nil {
				return nil, err
			}
			config.senates = append(config.senates, &database.SenateWeight{
				Weight:     weight,
				ScriptHash: *scriptHash,
			})
		}
		return &config, nil
	case GovernVersionClass:
		versionJson := GovernVersionJson{}
		err := json.Unmarshal(payload, versionJson)
		if err != nil {
			return nil, err
		}
		newVersion, err := version.NewVersionFromString(versionJson.version)
		if err != nil {
			return nil, err
		}
		config := GovernVersionConfig{
			blockHeight:  blockHeight,
			activeHeight: versionJson.activeHeight,
			shadow:       false,
			txSha:        txSha,
			version:      newVersion,
		}
		return &config, nil
	case GovernSupperClass:
		supperAddressJson := GovernSupperAddressJson{}
		err := json.Unmarshal(payload, supperAddressJson)
		if err != nil {
			return nil, err
		}
		config := GovernSupperConfig{
			blockHeight:  blockHeight,
			activeHeight: supperAddressJson.activeHeight,
			shadow:       false,
			txSha:        txSha,
			addresses:    make(map[wire.Hash]uint16),
		}
		for scriptHashString, id := range supperAddressJson.addresses {
			scriptHashBytes, err := hex.DecodeString(scriptHashString)
			if err != nil {
				return nil, err
			}
			scriptHash, err := wire.NewHash(scriptHashBytes)
			if err != nil {
				return nil, err
			}
			config.addresses[*scriptHash] = id
		}
		return &config, nil
	default:
		{
			return nil, fmt.Errorf("unsupported config class")
		}
	}
}
