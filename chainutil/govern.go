package chainutil

import (
	"fmt"
	"github.com/Sukhavati-Labs/go-miner/poc/pocutil/crypto/sha256"
	"github.com/Sukhavati-Labs/go-miner/version"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

//  govern config
//  Govern Config Class
const (
	GovernSupperClass  uint16 = 0x0000 // 0
	GovernVersionClass uint16 = 0x0001 // 1
	GovernSenateClass  uint16 = 0x0002 // 2
)

// UnsupportedGovernClassError describes an error where a govern config
// decoded has an unsupported class.
type UnsupportedGovernClassError uint16

func (e UnsupportedGovernClassError) Error() string {
	return fmt.Sprintf("unsupported govern class: %d", e)
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
	ConfigData() []byte
}

// DecodeGovernConfig decides the tx payload of govern config data and returns
// the GovernConfig
func DecodeGovernConfig(class uint16, blockHeight uint64, txSha *wire.Hash, payload []byte) (GovernConfig, error) {
	switch class {
	case GovernSupperClass:
		return newGovernSupperConfig(blockHeight, txSha, payload)
	case GovernVersionClass:
		return newGovernVersionConfig(blockHeight, txSha, payload)
	case GovernSenateClass:
		return newGovernSenateConfig(blockHeight, txSha, payload)
	default:
		return nil, UnsupportedGovernClassError(class)
	}
}

type SenateWeight struct {
	Weight     uint64
	ScriptHash [sha256.Size]byte
}

type GovernSenateConfig struct {
	blockHeight  uint64
	activeHeight uint64
	shadow       bool
	txSha        *wire.Hash
	senates      []SenateWeight
}

func (gsc GovernSenateConfig) GovernClass() uint16 {
	return GovernSenateClass
}

func (gsc GovernSenateConfig) BlockHeight() uint64 {
	return gsc.blockHeight
}

func (gsc GovernSenateConfig) ActiveHeight() uint64 {
	return gsc.activeHeight
}

func (gsc GovernSenateConfig) IsShadow() bool {
	return gsc.shadow
}

func (gsc GovernSenateConfig) TxSha() *wire.Hash {
	return gsc.txSha
}

func (gsc GovernSenateConfig) String() string {
	panic("implement me")
}

func (gsc GovernSenateConfig) ConfigData() []byte {
	panic("implement me")
}

func newGovernSenateConfig(blockHeight uint64, txSha *wire.Hash, payload []byte) (*GovernSenateConfig, error) {
	return nil, nil
}

type GovernVersionConfig struct {
	blockHeight  uint64
	activeHeight uint64
	shadow       bool
	txSha        *wire.Hash
	version      *version.Version
}

func (gvc GovernVersionConfig) GovernClass() uint16 {
	panic("implement me")
}

func (gvc GovernVersionConfig) BlockHeight() uint64 {
	panic("implement me")
}

func (gvc GovernVersionConfig) ActiveHeight() uint64 {
	panic("implement me")
}

func (gvc GovernVersionConfig) IsShadow() bool {
	panic("implement me")
}

func (gvc GovernVersionConfig) TxSha() *wire.Hash {
	panic("implement me")
}

func (gvc GovernVersionConfig) String() string {
	panic("implement me")
}

func (gvc GovernVersionConfig) ConfigData() []byte {
	panic("implement me")
}

func newGovernVersionConfig(blockHeight uint64, txSha *wire.Hash, payload []byte) (*GovernVersionConfig, error) {
	return nil, nil
}

type GovernSupperConfig struct {
	blockHeight  uint64
	activeHeight uint64
	shadow       bool
	txSha        *wire.Hash
	addresses    map[wire.Hash]uint32
}

func (gsc GovernSupperConfig) GovernClass() uint16 {
	panic("implement me")
}

func (gsc GovernSupperConfig) BlockHeight() uint64 {
	panic("implement me")
}

func (gsc GovernSupperConfig) ActiveHeight() uint64 {
	panic("implement me")
}

func (gsc GovernSupperConfig) IsShadow() bool {
	panic("implement me")
}

func (gsc GovernSupperConfig) TxSha() *wire.Hash {
	panic("implement me")
}

func (gsc GovernSupperConfig) String() string {
	panic("implement me")
}

func (gsc GovernSupperConfig) ConfigData() []byte {
	panic("implement me")
}

func newGovernSupperConfig(blockHeight uint64, txSha *wire.Hash, payload []byte) (*GovernSupperConfig, error) {
	return nil, nil
}
