package rpc

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/logging"
	pb "github.com/Sukhavati-Labs/go-miner/rpc/proto"
	"github.com/Sukhavati-Labs/go-miner/wire"
	"golang.org/x/net/context"
	"google.golang.org/grpc/status"
)

const (
	blockIDHeight = iota
	blockIDHash
)

var ErrInvalidBlockID = errors.New("invalid id for block")

var (
	RegexpBlockHeightPattern = `^height-\d+$`
	RegexpBlockHashPattern   = `^hash-[a-fA-F0-9]{64}$`
	RegexpBlockHeight        *regexp.Regexp
	RegexpBlockHash          *regexp.Regexp
)

func init() {
	var err error
	RegexpBlockHeight, err = regexp.Compile(RegexpBlockHeightPattern)
	if err != nil {
		panic(err)
	}
	RegexpBlockHash, err = regexp.Compile(RegexpBlockHashPattern)
	if err != nil {
		panic(err)
	}
}

func (s *Server) GetBlockV2(ctx context.Context, in *pb.GetBlockRequestV2) (*pb.GetBlockResponseV2, error) {
	var reqID = generateReqID()
	logging.CPrint(logging.INFO, "GetBlockV2 called", logging.LogFormat{"req_id": reqID})
	defer logging.CPrint(logging.INFO, "GetBlockV2 responded", logging.LogFormat{"req_id": reqID})

	if err := checkBlockIDLen(in.Id); err != nil {
		return nil, err
	}

	block, err := s.getBlockByID(in.Id)
	if err != nil {
		return nil, err
	}

	resp, err := s.marshalGetBlockV2Response(block)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *Server) GetBlockHeaderV2(ctx context.Context, in *pb.GetBlockRequestV2) (*pb.GetBlockHeaderResponse, error) {
	var reqID = generateReqID()
	logging.CPrint(logging.INFO, "GetBlockHeaderV2 called", logging.LogFormat{"req_id": reqID})
	defer logging.CPrint(logging.INFO, "GetBlockHeaderV2 responded", logging.LogFormat{"req_id": reqID})

	if err := checkBlockIDLen(in.Id); err != nil {
		return nil, err
	}

	block, err := s.getBlockByID(in.Id)
	if err != nil {
		return nil, err
	}

	resp, err := s.marshalGetBlockHeaderResponse(block)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *Server) GetBlockVerbose1V2(ctx context.Context, in *pb.GetBlockRequestV2) (*pb.GetBlockResponse, error) {
	var reqID = generateReqID()
	logging.CPrint(logging.INFO, "GetBlockVerbose1V2 called", logging.LogFormat{"req_id": reqID})
	defer logging.CPrint(logging.INFO, "GetBlockVerbose1V2 responded", logging.LogFormat{"req_id": reqID})

	if err := checkBlockIDLen(in.Id); err != nil {
		return nil, err
	}

	block, err := s.getBlockByID(in.Id)
	if err != nil {
		return nil, err
	}

	resp, err := s.marshalGetBlockResponse(block)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *Server) marshalGetBlockResponse(block *chainutil.Block) (*pb.GetBlockResponse, error) {
	idx := block.Height()
	maxIdx := s.chain.BestBlockHeight()
	var shaNextStr string
	if idx < maxIdx {
		shaNext, err := s.chain.GetBlockHashByHeight(idx + 1)
		if err != nil {
			logging.CPrint(logging.ERROR, "failed to query next block hash according to the block height", logging.LogFormat{"height": idx, "error": err})
			st := status.New(ErrAPIBlockHashByHeight, ErrCode[ErrAPIBlockHashByHeight])
			return nil, st.Err()
		}
		shaNextStr = shaNext.String()
	}
	blockHeader := &block.MsgBlock().Header
	blockHash := block.Hash().String()

	banList := make([]string, 0, len(blockHeader.BanList))
	for _, pk := range blockHeader.BanList {
		banList = append(banList, hex.EncodeToString(pk.SerializeCompressed()))
	}

	var proof = blockHeader.Proof
	blockReply := &pb.GetBlockResponse{
		Hash:            blockHash,
		ChainId:         blockHeader.ChainID.String(),
		Version:         blockHeader.Version,
		Height:          idx,
		Confirmations:   maxIdx + 1 - idx,
		Time:            blockHeader.Timestamp.Unix(),
		PreviousHash:    blockHeader.Previous.String(),
		NextHash:        shaNextStr,
		TransactionRoot: blockHeader.TransactionRoot.String(),
		WitnessRoot:     blockHeader.WitnessRoot.String(),
		ProposalRoot:    blockHeader.ProposalRoot.String(),
		Target:          blockHeader.Target.Text(16),
		Quality:         block.MsgBlock().Header.Quality().Text(16),
		Challenge:       hex.EncodeToString(blockHeader.Challenge.Bytes()),
		PublicKey:       hex.EncodeToString(blockHeader.PubKey.SerializeCompressed()),
		Proof:           &pb.Proof{X: hex.EncodeToString(proof.X), XPrime: hex.EncodeToString(proof.XPrime), BitLength: uint32(proof.BitLength)},
		BlockSignature:  &pb.PoCSignature{R: hex.EncodeToString(blockHeader.Signature.R.Bytes()), S: hex.EncodeToString(blockHeader.Signature.S.Bytes())},
		BanList:         banList,
		BlockSize:       uint32(block.Size()),
		TimeUtc:         blockHeader.Timestamp.UTC().Format(time.RFC3339),
		TxCount:         uint32(len(block.Transactions())),
	}
	proposalArea := block.MsgBlock().Proposals
	punishments := createFaultPubKeyResult(proposalArea.PunishmentArea)
	others := createNormalProposalResult(proposalArea.OtherArea)

	blockReply.ProposalArea = &pb.ProposalArea{
		PunishmentArea: punishments,
		OtherArea:      others,
	}

	txns := block.Transactions()
	rawTxns := make([]*pb.TxRawResult, len(txns))
	for i, tx := range txns {
		rawTxn, err := s.createTxRawResult(&config.ChainParams, tx.MsgTx(), tx.Hash().String(), blockHeader,
			blockHash, idx, maxIdx, false)
		if err != nil {
			logging.CPrint(logging.ERROR, "failed to query transactions in the block", logging.LogFormat{
				"height":   idx,
				"error":    err,
				"function": "GetBlock",
			})
			return nil, err
		}
		rawTxns[i] = rawTxn
	}
	blockReply.RawTx = rawTxns

	return blockReply, nil
}

func (s *Server) marshalGetBlockV2Response(block *chainutil.Block) (*pb.GetBlockResponseV2, error) {
	idx := block.Height()
	maxIdx := s.chain.BestBlockHeight()
	var shaNextStr string
	if idx < maxIdx {
		shaNext, err := s.chain.GetBlockHashByHeight(idx + 1)
		if err != nil {
			logging.CPrint(logging.ERROR, "failed to query next block hash according to the block height", logging.LogFormat{"height": idx, "error": err})
			st := status.New(ErrAPIBlockHashByHeight, ErrCode[ErrAPIBlockHashByHeight])
			return nil, st.Err()
		}
		shaNextStr = shaNext.String()
	}
	blockHeader := &block.MsgBlock().Header
	blockHash := block.Hash().String()

	banList := make([]string, 0, len(blockHeader.BanList))
	for _, pk := range blockHeader.BanList {
		banList = append(banList, hex.EncodeToString(pk.SerializeCompressed()))
	}

	txns := block.Transactions()
	var proof = blockHeader.Proof
	blockReply := &pb.GetBlockResponseV2{
		Hash:            blockHash,
		ChainId:         blockHeader.ChainID.String(),
		Version:         blockHeader.Version,
		Height:          idx,
		Confirmations:   maxIdx + 1 - idx,
		Timestamp:       blockHeader.Timestamp.Unix(),
		Previous:        blockHeader.Previous.String(),
		Next:            shaNextStr,
		TransactionRoot: blockHeader.TransactionRoot.String(),
		WitnessRoot:     blockHeader.WitnessRoot.String(),
		ProposalRoot:    blockHeader.ProposalRoot.String(),
		Target:          blockHeader.Target.Text(16),
		Quality:         block.MsgBlock().Header.Quality().Text(16),
		Challenge:       hex.EncodeToString(blockHeader.Challenge.Bytes()),
		PublicKey:       hex.EncodeToString(blockHeader.PubKey.SerializeCompressed()),
		Proof:           &pb.Proof{X: hex.EncodeToString(proof.X), XPrime: hex.EncodeToString(proof.XPrime), BitLength: uint32(proof.BitLength)},
		Signature:       &pb.PoCSignature{R: hex.EncodeToString(blockHeader.Signature.R.Bytes()), S: hex.EncodeToString(blockHeader.Signature.S.Bytes())},
		BanList:         banList,
		PlainSize:       uint32(block.Size()),
		PacketSize:      uint32(block.PacketSize()),
		TimeUtc:         blockHeader.Timestamp.UTC().Format(time.RFC3339),
		TxCount:         uint32(len(txns)),
	}
	proposalArea := block.MsgBlock().Proposals
	punishments := createFaultPubKeyResult(proposalArea.PunishmentArea)
	others := createNormalProposalResult(proposalArea.OtherArea)

	blockReply.Proposals = &pb.ProposalArea{
		PunishmentArea: punishments,
		OtherArea:      others,
	}

	txIDs := make([]string, len(txns))
	for i, tx := range txns {
		txIDs[i] = tx.Hash().String()
	}
	blockReply.Txids = txIDs

	return blockReply, nil
}

func (s *Server) marshalGetBlockHeaderResponse(block *chainutil.Block) (*pb.GetBlockHeaderResponse, error) {
	maxIdx := s.chain.BestBlockHeight()

	var shaNextStr string
	idx := block.Height()
	if idx < maxIdx {
		shaNext, err := s.chain.GetBlockHashByHeight(idx + 1)
		if err != nil {
			logging.CPrint(logging.ERROR, "failed to query next block hash according to the block height", logging.LogFormat{"height": idx, "error": err})
			st := status.New(ErrAPIBlockHashByHeight, ErrCode[ErrAPIBlockHashByHeight])
			return nil, st.Err()
		}
		shaNextStr = shaNext.String()
	}

	var proof = block.MsgBlock().Header.Proof

	msgBlock := block.MsgBlock()

	banList := make([]string, 0, len(msgBlock.Header.BanList))
	for _, pk := range msgBlock.Header.BanList {
		banList = append(banList, hex.EncodeToString(pk.SerializeCompressed()))
	}

	quality := msgBlock.Header.Quality().Text(10)

	blockHeaderReply := &pb.GetBlockHeaderResponse{
		Hash:            block.Hash().String(),
		ChainId:         msgBlock.Header.ChainID.String(),
		Version:         msgBlock.Header.Version,
		Height:          uint64(block.Height()),
		Confirmations:   uint64(1 + maxIdx - block.Height()),
		Timestamp:       msgBlock.Header.Timestamp.Unix(),
		PreviousHash:    msgBlock.Header.Previous.String(),
		NextHash:        shaNextStr,
		TransactionRoot: msgBlock.Header.TransactionRoot.String(),
		ProposalRoot:    msgBlock.Header.ProposalRoot.String(),
		Target:          fmt.Sprintf("%x", msgBlock.Header.Target),
		Challenge:       hex.EncodeToString(msgBlock.Header.Challenge.Bytes()),
		PublicKey:       hex.EncodeToString(msgBlock.Header.PubKey.SerializeCompressed()),
		Proof:           &pb.Proof{X: hex.EncodeToString(proof.X), XPrime: hex.EncodeToString(proof.XPrime), BitLength: uint32(proof.BitLength)},
		BlockSignature:  &pb.PoCSignature{R: hex.EncodeToString(msgBlock.Header.Signature.R.Bytes()), S: hex.EncodeToString(msgBlock.Header.Signature.S.Bytes())},
		BanList:         banList,
		Quality:         quality,
		TimeUtc:         msgBlock.Header.Timestamp.UTC().Format(time.RFC3339),
	}
	return blockHeaderReply, nil
}

func (s *Server) getBlockByID(id string) (*chainutil.Block, error) {
	if height, err := decodeBlockID(id, blockIDHeight); err == nil {
		return s.chain.GetBlockByHeight(height.(uint64))
	}

	if hash, err := decodeBlockID(id, blockIDHash); err == nil {
		return s.chain.GetBlockByHash(hash.(*wire.Hash))
	}

	return nil, ErrInvalidBlockID
}

func decodeBlockID(id string, typ int) (interface{}, error) {
	switch typ {
	case blockIDHeight:
		if !RegexpBlockHeight.MatchString(id) {
			return nil, ErrInvalidBlockID
		}
		height, _ := strconv.Atoi(id[7:]) // already make sure
		return uint64(height), nil

	case blockIDHash:
		if !RegexpBlockHash.MatchString(id) {
			return nil, ErrInvalidBlockID
		}
		return wire.NewHashFromStr(id[5:])

	default:
		return nil, ErrInvalidBlockID
	}
}
