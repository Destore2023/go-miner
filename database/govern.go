package database

import (
	"crypto/sha256"
)

type SenateWeight struct {
	Weight     uint64            // weight
	ScriptHash [sha256.Size]byte // The income address script of the equity
}

//func DecodeGovernanceConfig(data []byte) (GovernConfig, error) {
//	size := len(data)
//	if size <= 8 {
//		return nil, errors.New("Illegal data ")
//	}
//	configType := int(binary.BigEndian.Uint32(data[:8]))
//	switch configType {
//	case GovernSenate:
//		{
//			g := new(GovernSenateNodesConfig)
//			err := json.Unmarshal(data, g)
//			if err != nil {
//				return nil, err
//			}
//			return GovernConfig(*g), nil
//		}
//	default:
//		return nil, errors.New("Unsupported type ")
//	}
//}
//
//// Easy to sort and read
//type SenateEquities []SenateEquity
//type GovernSenateNodesConfig struct {
//	Height         uint64         `json:"height"`
//	SenateEquities SenateEquities `json:"senate_equities"`
//}
//
//func (config GovernSenateNodesConfig) Bytes() ([]byte, error) {
//	return json.Marshal(config)
//}
//
//func (config GovernSenateNodesConfig) GetConfigType() int {
//	return GovernSenate
//}
//
//func (config GovernSenateNodesConfig) GetHeight() uint64 {
//	return 0
//}
//
//func (config GovernSenateNodesConfig) GetBlockHeight() uint64 {
