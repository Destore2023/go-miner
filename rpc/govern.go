package rpc

import (
	"context"
	"errors"

	pb "github.com/Sukhavati-Labs/go-miner/rpc/proto"
)

func (s *Server) GetGovernConfig(ctx context.Context, in *pb.GetGovernConfigRequest) (*pb.GetGovernConfigResponse, error) {
	id := in.Id
	response := pb.GetGovernConfigResponse{
		Id: id,
	}
	switch id {
	//case database.GovernSenate:
	//	conf, err := s.db.FetchEnabledGovernanceConfig(id)
	//	if err != nil {
	//		return nil, err
	//	}
	//	nodesConfig, ok := conf.(database.GovernSenateNodesConfig)
	//	if !ok || len(nodesConfig.SenateEquities) == 0 {
	//		return nil, errors.New("error config type")
	//	}
	//	response.ActivateHeight = 0
	//	response.BlockHeight = 0
	//	nodes := make([]*pb.GovernSenateNode, len(nodesConfig.SenateEquities))
	//	for i, equity := range nodesConfig.SenateEquities {
	//		address, err := chainutil.NewAddressWitnessScriptHash(equity.ScriptHash[:], &config.ChainParams)
	//		if err != nil {
	//			return nil, err
	//		}
	//		nodes[i] = &pb.GovernSenateNode{
	//			Address:  address.EncodeAddress(),
	//			Weight: equity.Equity,
	//		}
	//	}
	//	senateNodesConfig := pb.GovernConfig{
	//		Config: &pb.GovernSenateConfig{
	//			Nodes: nodes,
	//		}
	//	}
	//	response.Config = &senateNodesConfig
	default:
		return nil, errors.New("Unknown configuration type ")
	}
	return &response, nil
}

func (s *Server) GetGovernConfigHistory(ctx context.Context, in *pb.GetGovernConfigHistoryRequest) (*pb.GetGovernConfigHistoryResponse, error) {
	return nil, nil
}
