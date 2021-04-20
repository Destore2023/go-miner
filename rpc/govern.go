package rpc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Sukhavati-Labs/go-miner/blockchain"

	pb "github.com/Sukhavati-Labs/go-miner/rpc/proto"
)

func isValidGovernId(id uint16) bool {
	if id >= blockchain.GovernSupperClass && id <= blockchain.GovernSenateClass {
		return true
	}
	return false
}

func getGovernConfig(config blockchain.GovernConfig) (*pb.GovernConfig, error) {
	id := config.GovernClass()
	switch id {
	case blockchain.GovernSupperClass:
		{
			supperConfig, ok := config.(*blockchain.GovernSupperConfig)
			if !ok {
				return nil, fmt.Errorf("error GovernSupperConfig")
			}
			addresses := make([]*pb.GovernSupperAddressInfo, 0)
			for addr, id := range supperConfig.GetAddresses() {
				addresses = append(addresses, &pb.GovernSupperAddressInfo{
					Id:      uint32(id),
					Address: hex.EncodeToString(addr[:]),
				})
			}
			resp := &pb.GovernConfig{
				Config: &pb.GovernConfig_GovernSupperConfig{
					GovernSupperConfig: &pb.GovernSupperConfig{
						Address: addresses,
					},
				},
			}
			return resp, nil
		}
	case blockchain.GovernVersionClass:
		{
			versionConfig, ok := config.(*blockchain.GovernVersionConfig)
			if !ok {
				return nil, fmt.Errorf("error GovernVersionConfig")
			}
			resp := &pb.GovernConfig{
				Config: &pb.GovernConfig_GovernVersionConfig{
					GovernVersionConfig: &pb.GovernVersionConfig{
						Version: versionConfig.GetVersion().String(),
					},
				},
			}
			return resp, nil
		}
	case blockchain.GovernSenateClass:
		{
			senateConfig, ok := config.(*blockchain.GovernSenateConfig)
			if !ok {
				return nil, fmt.Errorf("error GovernSenateConfig")
			}
			cs := make([]*pb.GovernSenateNode, 0)
			for _, node := range senateConfig.GetNodes() {
				cs = append(cs, &pb.GovernSenateNode{Weight: node.Weight, Address: hex.EncodeToString(node.ScriptHash[:])})
			}
			resp := &pb.GovernConfig{
				Config: &pb.GovernConfig_GovernSenateConfig{
					GovernSenateConfig: &pb.GovernSenateConfig{
						Nodes: cs,
					},
				},
			}
			return resp, nil
		}
	default:
		return nil, errors.New("Unknown configuration type ")
	}
}
func (s *Server) GetGovernConfig(ctx context.Context, in *pb.GetGovernConfigRequest) (*pb.GetGovernConfigResponse, error) {
	id := in.Id

	if !isValidGovernId(uint16(id)) {
		return nil, fmt.Errorf("error govern type")
	}
	config, err := s.chain.FetchEnabledGovernConfig(uint16(id))
	if err != nil {
		return nil, err
	}
	response := pb.GetGovernConfigResponse{
		Id:             uint32(config.GovernClass()),
		BlockHeight:    config.BlockHeight(),
		ActivateHeight: config.ActiveHeight(),
		TxId:           config.TxSha().String(),
	}
	governConfig, err := getGovernConfig(config)
	if err != nil {
		return nil, err
	}
	response.Config = governConfig
	return &response, nil
}

func (s *Server) GetGovernConfigHistory(ctx context.Context, in *pb.GetGovernConfigHistoryRequest) (*pb.GetGovernConfigHistoryResponse, error) {
	if !isValidGovernId(uint16(in.Id)) {
		return nil, fmt.Errorf("error govern type")
	}
	configs, err := s.chain.FetchGovernConfig(in.Id, in.IncludeShadow)
	if err != nil {
		return nil, err
	}
	response := &pb.GetGovernConfigHistoryResponse{
		Configs: make([]*pb.GovernConfig, 0),
	}
	for _, config := range configs {
		governConfig, err := getGovernConfig(config)
		if err != nil {
			return nil, err
		}
		response.Configs = append(response.Configs, governConfig)
	}
	return response, nil
}
