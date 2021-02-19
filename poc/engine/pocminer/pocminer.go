package pocminer

import (
	"errors"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
)

type PoCMiner interface {
	Start() error
	Stop() error
	Started() bool
	Type() string
	SetPayoutAddresses(addresses []chainutil.Address) error
}

var (
	ErrInvalidMinerType = errors.New("invalid Miner type")
	ErrInvalidMinerArgs = errors.New("invalid Miner args")
	ErrUnimplemented    = errors.New("unimplemented Miner interface")
)

var (
	BackendList []MinerBackend
)

type MinerBackend struct {
	Typ         string
	NewPoCMiner func(args ...interface{}) (PoCMiner, error)
}

func AddPoCMinerBackend(ins MinerBackend) {
	for _, kb := range BackendList {
		if kb.Typ == ins.Typ {
			return
		}
	}
	BackendList = append(BackendList, ins)
}

func NewPoCMiner(kbType string, args ...interface{}) (PoCMiner, error) {
	for _, kb := range BackendList {
		if kb.Typ == kbType {
			return kb.NewPoCMiner(args...)
		}
	}
	return nil, ErrInvalidMinerType
}
