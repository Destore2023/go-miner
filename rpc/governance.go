package rpc

import (
	"context"
	"errors"

	"github.com/Sukhavati-Labs/go-miner/chainutil"

	"github.com/Sukhavati-Labs/go-miner/config"

	"github.com/Sukhavati-Labs/go-miner/database"
	pb "github.com/Sukhavati-Labs/go-miner/rpc/proto"
)

func (s *Server) GetGovernanceConfig(ctx context.Context, in *pb.GetGovernanceConfigRequest) (*pb.GetGovernanceConfigResponse, error) {
	configType := in.ConfigType
	response := pb.GetGovernanceConfigResponse{
		ConfigType: configType,
	}
	switch configType {
	case database.GovernanceSenate:
		conf, err := s.db.FetchEnabledGovernanceConfig(int(configType))
		if err != nil {
			return nil, err
		}
		nodesConfig, ok := conf.(database.GovernanceSenateNodesConfig)
		if !ok || len(nodesConfig.SenateEquities) == 0 {
			return nil, errors.New("error config type")
		}
		response.Height = 0
		nodes := make([]*pb.GovernanceSenateNode, len(nodesConfig.SenateEquities))
		for i, equity := range nodesConfig.SenateEquities {
			address, err := chainutil.NewAddressWitnessScriptHash(equity.ScriptHash[:], &config.ChainParams)
			if err != nil {
				return nil, err
			}
			nodes[i] = &pb.GovernanceSenateNode{
				Address:  address.EncodeAddress(),
				Equities: equity.Equity,
			}
		}
		senateNodesConfig := pb.GetGovernanceConfigResponse_SenateNodesConfig{SenateNodesConfig: &pb.GovernanceSenateConfig{
			Nodes: nodes,
		}}
		response.Config = &senateNodesConfig
	default:
		return nil, errors.New("Unknown configuration type ")
	}
	return &response, nil
}
