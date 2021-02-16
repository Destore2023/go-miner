package database

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
)

const (
	// senate note config
	GovernanceSenate = iota
)

const ConfigEnabled = byte(1)
const ConfigDisabled = byte(0)

type GovernanceConfig interface {
	GetConfigType() int
	GetHeight() uint64 //Effective height
	Bytes() ([]byte, error)
	IsEnabled() bool
}
type SenateEquity struct {
	Equity     uint64            // sum 10000
	ScriptHash [sha256.Size]byte // The income address script of the equity
}

// +---------------------------------------
// |  magic |  type  |    |
// +---------------------------------------
func DecodeGovernanceConfig(data []byte) (GovernanceConfig, error) {
	size := len(data)
	if size <= 8 {
		return nil, errors.New("Illegal data ")
	}
	configType := int(binary.BigEndian.Uint32(data[:8]))
	switch configType {
	case GovernanceSenate:
		{
			g := new(GovernanceSenateNodesConfig)
			err := json.Unmarshal(data, g)
			if err != nil {
				return nil, err
			}
			return GovernanceConfig(*g), nil
		}
	default:
		return nil, errors.New("Unsupported type ")
	}
}

// Easy to sort and read
type SenateEquities []SenateEquity
type GovernanceSenateNodesConfig struct {
	Height         uint64         `json:"height"`
	SenateEquities SenateEquities `json:"senate_equities"`
	enabled        bool           `json:"enabled"`
}

func (config GovernanceSenateNodesConfig) Bytes() ([]byte, error) {
	return json.Marshal(config)
}

func (config GovernanceSenateNodesConfig) GetConfigType() int {
	return GovernanceSenate
}

func (config GovernanceSenateNodesConfig) GetHeight() uint64 {
	return 0
}

func (config GovernanceSenateNodesConfig) IsEnabled() bool {
	return config.enabled
}
