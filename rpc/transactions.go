package rpc

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Sukhavati-Labs/go-miner/blockchain"
	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/consensus"
	"github.com/Sukhavati-Labs/go-miner/logging"
	pb "github.com/Sukhavati-Labs/go-miner/rpc/proto"
	"github.com/Sukhavati-Labs/go-miner/txscript"
	"github.com/Sukhavati-Labs/go-miner/wire"
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc/status"
	"reflect"
	"sort"
)

func (s *Server) GetCoinbase(ctx context.Context, in *pb.GetCoinbaseRequest) (*pb.GetCoinbaseResponse, error) {
	block, err := s.chain.GetBlockByHeight(in.Height)
	if err != nil {
		logging.CPrint(logging.ERROR, "GetCoinbase called", logging.LogFormat{"height": in.Height, "error": err})
		return nil, err
	}

	currentHeight := s.chain.BestBlockHeight()

	txs := block.Transactions()
	if len(txs) <= 0 {
		logging.CPrint(logging.ERROR, "GetCoinbase called,cannot find transaction in block")
		return nil, errors.New("cannot find transaction in block")
	}
	coinbase := txs[0]
	msgTx := coinbase.MsgTx()

	vins, bindingValue, err := s.showCoinbaseInputDetails(msgTx)
	if err != nil {
		logging.CPrint(logging.ERROR, "GetCoinbase called, call showCoinbaseInputDetails ", logging.LogFormat{"height": in.Height, "error": err})
		return nil, err
	}
	bindingValueStr, err := AmountToString(bindingValue)
	if err != nil {
		logging.CPrint(logging.ERROR, "GetCoinbase called, call AmountToString ", logging.LogFormat{"height": in.Height, "error": err})
		return nil, err
	}

	vouts, totalFees, err := s.showCoinbaseOutputDetails(msgTx, &config.ChainParams, block.Height(), bindingValue, block.MsgBlock().Header.Proof.BitLength)
	if err != nil {
		logging.CPrint(logging.ERROR, "GetCoinbase called, call showCoinbaseOutputDetails ", logging.LogFormat{"height": in.Height, "error": err})
		return nil, err
	}

	confirmations := currentHeight - block.Height()
	var txStatus int32
	if confirmations >= consensus.CoinbaseMaturity {
		txStatus = txStatusConfirmed
	} else {
		txStatus = txStatusConfirming
	}
	feesStr, err := AmountToString(totalFees)
	if err != nil {
		logging.CPrint(logging.ERROR, "GetCoinbase called, call totalFees AmountToString ", logging.LogFormat{"height": in.Height, "error": err})
		return nil, err
	}

	return &pb.GetCoinbaseResponse{
		TxId:     msgTx.TxHash().String(),
		Version:  msgTx.Version,
		LockTime: msgTx.LockTime,
		Block: &pb.BlockInfoForTx{
			Height:    block.Height(),
			BlockHash: block.Hash().String(),
			Timestamp: block.MsgBlock().Header.Timestamp.Unix(),
		},
		BindingValue:  bindingValueStr,
		Vin:           vins,
		Vout:          vouts,
		Payload:       hex.EncodeToString(msgTx.Payload),
		Confirmations: confirmations,
		TxSize:        uint32(msgTx.PlainSize()),
		TotalFees:     feesStr,
		Status:        txStatus,
	}, nil
}

func (s *Server) GetTxPool(ctx context.Context, in *empty.Empty) (*pb.GetTxPoolResponse, error) {
	var reqID = generateReqID()
	logging.CPrint(logging.INFO, "GetTxPool called", logging.LogFormat{"req_id": reqID})
	defer logging.CPrint(logging.INFO, "GetTxPool responded", logging.LogFormat{"req_id": reqID})

	resp := &pb.GetTxPoolResponse{}
	if err := s.marshalGetTxPoolResponse(reflect.ValueOf(resp), -1); err != nil {
		logging.CPrint(logging.ERROR, "GetTxPool fail on marshalGetTxPoolResponse", logging.LogFormat{"err": err})
		return nil, err
	}
	return resp, nil
}

func (s *Server) GetTxPoolVerbose0(ctx context.Context, in *empty.Empty) (*pb.GetTxPoolVerbose0Response, error) {
	var reqID = generateReqID()
	logging.CPrint(logging.INFO, "GetTxPoolVerbose0 called", logging.LogFormat{"req_id": reqID})
	defer logging.CPrint(logging.INFO, "GetTxPoolVerbose0 responded", logging.LogFormat{"req_id": reqID})

	resp := &pb.GetTxPoolVerbose0Response{}
	if err := s.marshalGetTxPoolResponse(reflect.ValueOf(resp), 0); err != nil {
		logging.CPrint(logging.ERROR, "GetTxPoolVerbose0 fail on marshalGetTxPoolResponse", logging.LogFormat{"err": err})
		return nil, err
	}
	return resp, nil
}

func (s *Server) GetTxPoolVerbose1(ctx context.Context, in *empty.Empty) (*pb.GetTxPoolVerbose1Response, error) {
	var reqID = generateReqID()
	logging.CPrint(logging.INFO, "GetTxPoolVerbose1 called", logging.LogFormat{"req_id": reqID})
	defer logging.CPrint(logging.INFO, "GetTxPoolVerbose1 responded", logging.LogFormat{"req_id": reqID})

	resp := &pb.GetTxPoolVerbose1Response{}
	err := s.marshalGetTxPoolResponse(reflect.ValueOf(resp), 1)
	if err != nil {
		logging.CPrint(logging.ERROR, "GetTxPoolVerbose1 fail on marshalGetTxPoolResponse", logging.LogFormat{"err": err})
		return nil, err
	}
	return resp, nil
}

func (s *Server) GetStakingTxPoolInfo(ctx context.Context, in *empty.Empty) (*pb.GetStakingTxPoolInfoResponse, error) {
	resp := &pb.GetStakingTxPoolInfoResponse{}
	return resp, nil
}

func (s *Server) GetStakingRewardRecord(ctx context.Context, in *pb.GetStakingRewardRecordRequest) (*pb.GetStakingRewardRecordResponse, error) {
	if in.Timestamp > 9999999999 {
		return nil, errors.New("error Timestamp")
	}
	queryTime := in.Timestamp
	if queryTime == 0 {
		queryTime = uint64(s.chain.BestBlockNode().Timestamp.Unix())
	}
	awardRecords, err := s.db.FetchStakingAwardedRecordByTime(queryTime)
	if err != nil {
		logging.CPrint(logging.ERROR, "GetStakingRewardRecord fail on FetchStakingAwardedRecordByTime", logging.LogFormat{"err": err, "timestamp": in.Timestamp})
		return nil, err
	}
	if len(awardRecords) == 0 {
		return &pb.GetStakingRewardRecordResponse{}, err
	}

	records := make([]*pb.StakingRewardRecord, len(awardRecords))
	for i, record := range awardRecords {
		transaction, err := s.getRawTransaction(record.TxId.String())
		if err != nil {
			return nil, err
		}
		records[i] = &pb.StakingRewardRecord{
			TxId:        record.TxId.String(),
			AwardedTime: record.AwardedTime,
			Tx:          transaction,
		}
	}
	resp := &pb.GetStakingRewardRecordResponse{
		Records: records,
	}
	return resp, nil
}

func (s *Server) getRawTransaction(txId string) (*pb.TxRawResult, error) {
	logging.CPrint(logging.INFO, "rpc: GetRawTransaction", logging.LogFormat{"txId": txId})
	err := checkTransactionIdLen(txId)
	if err != nil {
		logging.CPrint(logging.ERROR, "getRawTransaction fail on checkTransactionIdLen", logging.LogFormat{"err": err, "txId": txId})
		return nil, err
	}

	// Convert the provided transaction hash hex to a Hash.
	txHash, err := wire.NewHashFromStr(txId)
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to decode the input string into hash", logging.LogFormat{
			"input string": txId,
			"error":        err})
		st := status.New(ErrAPIInvalidTxHex, ErrCode[ErrAPIInvalidTxHex])
		return nil, st.Err()
	}

	// Try to fetch the transaction from the memory pool and if that fails,
	// try the block database.
	var mtx *wire.MsgTx
	var chainHeight uint64
	var blockHeader *wire.BlockHeader

	tx, err := s.txMemPool.FetchTransaction(txHash)
	if err != nil {
		txList, err := s.chain.GetTransactionInDB(txHash)
		if err != nil || len(txList) == 0 {
			logging.CPrint(logging.ERROR, "failed to query the transaction information in txPool or database according to the transaction hash",
				logging.LogFormat{
					"hash":  txHash.String(),
					"error": err,
				})
			st := status.New(ErrAPINoTxInfo, ErrCode[ErrAPINoTxInfo])
			return nil, st.Err()
		}

		lastTx := txList[len(txList)-1]
		if lastTx.Err != nil {
			logging.CPrint(logging.ERROR, "failed to query the transaction information in txPool or database according to the transaction hash,last tx with error",
				logging.LogFormat{
					"hash":  txHash.String(),
					"error": lastTx.Err,
				})
			return nil, lastTx.Err
		}
		blockSha := lastTx.BlockSha
		mtx = lastTx.Tx
		// query block header
		blockHeader, err = s.chain.GetHeaderByHash(blockSha)
		if err != nil {
			logging.CPrint(logging.ERROR, "failed to query the block header according to the block hash",
				logging.LogFormat{
					"block": lastTx.BlockSha.String(),
					"error": err,
				})
			st := status.New(ErrAPIBlockHeaderNotFound, ErrCode[ErrAPIBlockHeaderNotFound])
			return nil, st.Err()
		}
		chainHeight = s.chain.BestBlockHeight()
	} else {
		mtx = tx.MsgTx()
	}

	rep, err := s.createTxRawResult(mtx, blockHeader, chainHeight, false)
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to query information of transaction according to the transaction hash",
			logging.LogFormat{
				"hash":  txId,
				"error": err,
			})
		st := status.New(ErrAPIRawTx, ErrCode[ErrAPIRawTx])
		return nil, st.Err()
	}
	logging.CPrint(logging.INFO, "rpc: GetRawTransaction completed", logging.LogFormat{"hash": txId})
	return rep, nil
}

func (s *Server) GetRawTransaction(ctx context.Context, in *pb.GetRawTransactionRequest) (*pb.TxRawResult, error) {
	return s.getRawTransaction(in.TxId)
}

func (s *Server) marshalGetTxPoolResponse(resp reflect.Value, verbose int) error {
	resp = reflect.Indirect(resp)
	txs := s.txMemPool.TxDescs()
	sort.Sort(sort.Reverse(txDescList(txs)))
	orphans := s.txMemPool.OrphanTxs()
	sort.Sort(sort.Reverse(orphanTxDescList(orphans)))
	var txsPlainSize, txsPacketSize, orphansPlainSize, orphansPacketSize int
	txIDs, orphanIDs := make([]string, 0, len(txs)), make([]string, 0, len(orphans))

	for _, tx := range txs {
		txsPlainSize += tx.Tx.PlainSize()
		txsPacketSize += tx.Tx.PacketSize()
		txIDs = append(txIDs, tx.Tx.Hash().String())
	}
	for _, orphan := range orphans {
		orphansPlainSize += orphan.PlainSize()
		orphansPacketSize += orphan.PacketSize()
		orphanIDs = append(orphanIDs, orphan.Hash().String())
	}

	// write common parts of GetTxPoolResponse, GetTxPoolResponseV0 and GetTxPoolResponseV1
	resp.FieldByName("TxCount").SetUint(uint64(len(txs)))
	resp.FieldByName("OrphanCount").SetUint(uint64(len(orphans)))
	resp.FieldByName("TxPlainSize").SetUint(uint64(txsPlainSize))
	resp.FieldByName("TxPacketSize").SetUint(uint64(txsPacketSize))
	resp.FieldByName("OrphanPlainSize").SetUint(uint64(orphansPlainSize))
	resp.FieldByName("OrphanPacketSize").SetUint(uint64(orphansPacketSize))
	resp.FieldByName("Txs").Set(reflect.ValueOf(txIDs))
	resp.FieldByName("Orphans").Set(reflect.ValueOf(orphanIDs))

	// write differential parts
	var err error
	switch verbose {
	case 0:
		txDescsV0 := make([]*pb.GetTxDescVerbose0Response, 0, len(txs))
		for _, tx := range txs {
			txResp := &pb.GetTxDescVerbose0Response{}
			if err = s.marshalGetTxDescResponse(reflect.ValueOf(txResp), tx, verbose); err != nil {
				logging.CPrint(logging.ERROR, "marshalGetTxPoolResponse error",
					logging.LogFormat{
						"verbose": verbose,
						"error":   err,
					})
				return err
			}
			txDescsV0 = append(txDescsV0, txResp)
		}
		orphanDescs := make([]*pb.GetOrphanTxDescResponse, 0, len(orphans))
		for _, orphan := range orphans {
			orphanResp := &pb.GetOrphanTxDescResponse{}
			if err = s.marshalGetOrphanTxDescResponse(reflect.ValueOf(orphanResp), orphan); err != nil {
				return err
			}
			orphanDescs = append(orphanDescs, orphanResp)
		}
		resp.FieldByName("TxDescs").Set(reflect.ValueOf(txDescsV0))
		resp.FieldByName("OrphanDescs").Set(reflect.ValueOf(orphanDescs))

	case 1:
		txDescsV1 := make([]*pb.GetTxDescVerbose1Response, 0, len(txs))
		for _, tx := range txs {
			txResp := &pb.GetTxDescVerbose1Response{}
			if err = s.marshalGetTxDescResponse(reflect.ValueOf(txResp), tx, verbose); err != nil {
				logging.CPrint(logging.ERROR, "marshalGetTxPoolResponse error",
					logging.LogFormat{
						"verbose": verbose,
						"error":   err,
					})
				return err
			}
			txDescsV1 = append(txDescsV1, txResp)
		}
		orphanDescs := make([]*pb.GetOrphanTxDescResponse, 0, len(orphans))
		for _, orphan := range orphans {
			orphanResp := &pb.GetOrphanTxDescResponse{}
			if err = s.marshalGetOrphanTxDescResponse(reflect.ValueOf(orphanResp), orphan); err != nil {
				logging.CPrint(logging.ERROR, "marshalGetTxPoolResponse error",
					logging.LogFormat{
						"verbose": verbose,
						"error":   err,
					})
				return err
			}
			orphanDescs = append(orphanDescs, orphanResp)
		}
		resp.FieldByName("TxDescs").Set(reflect.ValueOf(txDescsV1))
		resp.FieldByName("OrphanDescs").Set(reflect.ValueOf(orphanDescs))
	}

	return nil
}

func (s *Server) marshalGetTxDescResponse(resp reflect.Value, txD *blockchain.TxDesc, verbose int) error {
	resp = reflect.Indirect(resp)
	startingPriority, _ := txD.StartingPriority()
	totalInputAge, _ := txD.TotalInputAge()

	// write common parts of GetTxDescV0Response, GetTxDescV1Response
	resp.FieldByName("TxId").SetString(txD.Tx.Hash().String())
	resp.FieldByName("PlainSize").SetUint(uint64(txD.Tx.PlainSize()))
	resp.FieldByName("PacketSize").SetUint(uint64(txD.Tx.PacketSize()))
	resp.FieldByName("Time").SetInt(txD.Added.UnixNano())
	resp.FieldByName("Height").SetUint(txD.Height)
	resp.FieldByName("Fee").SetString(txD.Fee.String())
	resp.FieldByName("StartingPriority").SetFloat(startingPriority)
	resp.FieldByName("TotalInputAge").SetInt(totalInputAge.IntValue())

	// write differential parts
	switch verbose {
	case 1:
		txStore := s.chain.FetchTransactionStore(txD.Tx, false)
		priority, err := txD.CurrentPriority(txStore, s.chain.BestBlockHeight())
		if err != nil {
			logging.CPrint(logging.ERROR, "error on rpc get txD current priority", logging.LogFormat{"err": err, "txId": txD.Tx.Hash()})
			return err
		}
		txIns := txD.Tx.MsgTx().TxIn
		depends := make([]*pb.TxOutPoint, 0, len(txIns))
		for _, txIn := range txIns {
			depends = append(depends, &pb.TxOutPoint{TxId: txIn.PreviousOutPoint.Hash.String(), Index: txIn.PreviousOutPoint.Index})
		}
		resp.FieldByName("CurrentPriority").SetFloat(priority)
		resp.FieldByName("Depends").Set(reflect.ValueOf(depends))
	}

	return nil
}

func (s *Server) marshalGetOrphanTxDescResponse(resp reflect.Value, orphan *chainutil.Tx) error {
	resp = reflect.Indirect(resp)
	resp.FieldByName("TxId").SetString(orphan.Hash().String())
	resp.FieldByName("PlainSize").SetUint(uint64(orphan.PlainSize()))
	resp.FieldByName("PacketSize").SetUint(uint64(orphan.PacketSize()))
	txIns := orphan.MsgTx().TxIn
	depends := make([]*pb.TxOutPoint, 0, len(txIns))
	for _, txIn := range txIns {
		depends = append(depends, &pb.TxOutPoint{TxId: txIn.PreviousOutPoint.Hash.String(), Index: txIn.PreviousOutPoint.Index})
	}
	resp.FieldByName("Depends").Set(reflect.ValueOf(depends))

	return nil
}

func (s *Server) showCoinbaseInputDetails(mtx *wire.MsgTx) ([]*pb.Vin, int64, error) {
	if mtx == nil {
		logging.CPrint(logging.ERROR, "showCoinbaseInputDetails mtx is nil")
		return nil, 0, fmt.Errorf("showCoinbaseInputDetails error, mtx is nil ")
	}
	vinList := make([]*pb.Vin, len(mtx.TxIn))
	var bindingValue int64
	if blockchain.IsCoinBaseTx(mtx) {
		return nil, 0, errors.New("showCoinbaseInputDetails error, mtx isn't a coinbase tx ")
	}
	txIn := mtx.TxIn[0]
	vinTemp := &pb.Vin{
		TxId:     txIn.PreviousOutPoint.Hash.String(),
		Sequence: txIn.Sequence,
		Witness:  txWitnessToHex(txIn.Witness),
	}
	vinList[0] = vinTemp

	for i, txIn := range mtx.TxIn[1:] {
		vinTemp := &pb.Vin{
			TxId:     txIn.PreviousOutPoint.Hash.String(),
			Vout:     txIn.PreviousOutPoint.Index,
			Sequence: txIn.Sequence,
		}

		originTx, err := s.chain.GetTransaction(&txIn.PreviousOutPoint.Hash)
		if err != nil {
			logging.CPrint(logging.ERROR, "showCoinbaseInputDetails GetTransaction with error", logging.LogFormat{"error": err})
			return nil, 0, err
		}
		bindingValue += originTx.TxOut[txIn.PreviousOutPoint.Index].Value
		vinList[i+1] = vinTemp
	}
	return vinList, bindingValue, nil
}

func (s *Server) showCoinbaseOutputDetails(mtx *wire.MsgTx, chainParams *config.Params, height uint64, bindingValue int64, bitlength int) ([]*pb.CoinbaseVout, int64, error) {
	if mtx == nil {
		return nil, 0, fmt.Errorf("showCoinbaseOutputDetails error, mtx is nil")
	}
	voutList := make([]*pb.CoinbaseVout, 0, len(mtx.TxOut))

	g, err := chainutil.NewAmountFromInt(bindingValue)
	if err != nil {
		logging.CPrint(logging.ERROR, "showCoinbaseInputDetails NewAmountFromInt with error", logging.LogFormat{"error": err})
		return nil, -1, err
	}

	//coinbasePayload := blockchain.NewCoinbasePayload()
	//err = coinbasePayload.SetBytes(mtx.Payload)
	//if err != nil {
	//	logging.CPrint(logging.ERROR, "failed to deserialize coinbase payload", logging.LogFormat{"error": err})
	//	return nil, -1, err
	//}
	//numStaking := coinbasePayload.NumStakingReward()

	baseMiner, superNode, _, err := blockchain.CalcBlockSubsidy(height, chainParams, g, bitlength)
	if err != nil {
		logging.CPrint(logging.ERROR, "showCoinbaseInputDetails CalcBlockSubsidy with error", logging.LogFormat{"error": err})
		return nil, -1, err
	}

	blockSubsidy, err := baseMiner.Add(superNode)
	if err != nil {
		logging.CPrint(logging.ERROR, "showCoinbaseInputDetails Calc base Miner reward with error", logging.LogFormat{"error": err})
		return nil, -1, err
	}

	var (
		outputType string
		totalOut   = chainutil.ZeroAmount()
	)
	for i, v := range mtx.TxOut {
		// The disassembled string will contain [error] inline if the
		// script doesn't fully parse, so ignore the error here.
		disBuf, err := txscript.DisasmString(v.PkScript)
		if err != nil {
			logging.CPrint(logging.WARN, "decode pkscript to asm exists err", logging.LogFormat{"err": err})
			st := status.New(ErrAPIDisasmScript, ErrCode[ErrAPIDisasmScript])
			return nil, -1, st.Err()
		}

		// Ignore the error here since an error means the script
		// couldn't parse and there is no additional information about
		// it anyways.
		scriptClass, addrs, _, reqSigs, err := txscript.ExtractPkScriptAddrs(
			v.PkScript, chainParams)
		if err != nil {
			st := status.New(ErrAPIExtractPKScript, ErrCode[ErrAPIExtractPKScript])
			return nil, -1, st.Err()
		}

		encodedAddrs := make([]string, len(addrs))
		for j, addr := range addrs {
			encodedAddrs[j] = addr.EncodeAddress()
		}

		//if uint32(i) < numStaking {
		//	outputType = "staking reward"
		//} else {
		//	outputType = "miner"
		//}

		valueStr, err := AmountToString(v.Value)
		if err != nil {
			logging.CPrint(logging.ERROR, "showCoinbaseInputDetails AmountToString with error", logging.LogFormat{"error": err})
			return nil, -1, err
		}

		totalOut, err = totalOut.AddInt(v.Value)
		if err != nil {
			logging.CPrint(logging.ERROR, "showCoinbaseInputDetails calc total out with error", logging.LogFormat{"error": err})
			return nil, -1, err
		}

		vout := &pb.CoinbaseVout{
			N:     uint32(i),
			Value: valueStr,
			ScriptPublicKey: &pb.ScriptPubKeyResult{
				Asm:       disBuf,
				Hex:       hex.EncodeToString(v.PkScript),
				ReqSigs:   uint32(reqSigs),
				Type:      scriptClass.String(),
				Addresses: encodedAddrs,
			},
			Type: outputType,
		}

		voutList = append(voutList, vout)
	}
	totalFees, err := totalOut.Sub(blockSubsidy)
	if err != nil {
		logging.CPrint(logging.ERROR, "showCoinbaseInputDetails calc total fees with error", logging.LogFormat{"error": err})
		return nil, -1, err
	}

	return voutList, totalFees.IntValue(), nil
}

func (s *Server) createTxRawResult(mtx *wire.MsgTx, blockHeader *wire.BlockHeader, chainHeight uint64, detail bool) (*pb.TxRawResult, error) {
	if mtx == nil {
		logging.CPrint(logging.ERROR, "failed to create raw tx , mtx is nil! ")
		return nil, fmt.Errorf("createTxRawResult get nil tx")
	}
	blockHeight := blockHeader.Height
	vouts, totalOutValue, err := createVoutList(mtx, &config.ChainParams, nil)
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to create vout list", logging.LogFormat{"error": err})
		return nil, err
	}
	to := make([]*pb.ToAddressForTx, 0)
	for _, voutR := range vouts {
		to = append(to, &pb.ToAddressForTx{
			Address: voutR.ScriptPublicKey.Addresses,
			Value:   voutR.Value,
		})
	}
	if mtx.Payload == nil {
		mtx.Payload = make([]byte, 0)
	}

	vins, fromAddrs, inputs, totalInValue, err := s.createVinList(mtx, &config.ChainParams)
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to create vin list", logging.LogFormat{"error": err})
		return nil, err
	}

	txId := mtx.TxHash()
	code, confirmations, err := s.getTxStatus(&txId)
	if err != nil {
		logging.CPrint(logging.ERROR, "Failed to getTxStatus ", logging.LogFormat{"err": err, "txId": txId})
		return nil, err
	}
	txType, err := s.getTxType(mtx)
	if err != nil {
		logging.CPrint(logging.ERROR, "Failed to getTxType ", logging.LogFormat{"err": err, "txId": txId})
		return nil, err
	}

	isCoinbase := blockchain.IsCoinBaseTx(mtx)
	fee := "0"
	if !isCoinbase {
		fee, err = AmountToString(totalInValue - totalOutValue)
		if err != nil {
			logging.CPrint(logging.ERROR, "Failed to transfer amount to string", logging.LogFormat{"err": err})
			return nil, err
		}
	}

	txReply := &pb.TxRawResult{
		TxId:          txId.String(),
		Version:       mtx.Version,
		LockTime:      mtx.LockTime,
		Vin:           vins,
		Vout:          vouts,
		FromAddress:   fromAddrs,
		To:            to,
		Inputs:        inputs,
		Payload:       hex.EncodeToString(mtx.Payload),
		TxSize:        uint32(mtx.PlainSize()),
		Fee:           fee,
		Gas:           "0",
		Status:        code,
		Type:          txType,
		Coinbase:      isCoinbase,
		TotalInValue:  totalInValue,
		TotalOutValue: totalInValue,
		Confirmations: confirmations,
	}

	if blockHeader != nil {
		// This is not a typo, they are identical in skt as well.
		txReply.Block = &pb.BlockInfoForTx{
			Height:    blockHeight,
			BlockHash: blockHeader.BlockHash().String(),
			Timestamp: blockHeader.Timestamp.Unix(),
		}
		txReply.Confirmations = 1 + chainHeight - blockHeight
	}
	if detail {
		// Hex
		bs, err := mtx.Bytes(wire.Packet)
		if err != nil {
			logging.CPrint(logging.ERROR, "createTxRawResult get tx bytes error", logging.LogFormat{"error": err})
			return nil, err
		}
		txReply.Hex = hex.EncodeToString(bs)
	}

	return txReply, nil
}

// Tx type codes are shown below:
//  -----------------------------------------------------
// |  Tx Type  | Staking | Binding | Ordinary | Coinbase |
// |-----------------------------------------------------
// | Type Code |    1    |    2    |     3    |     4    |
//   ----------------------------------------------------
func (s *Server) getTxType(tx *wire.MsgTx) (int32, error) {
	if tx == nil {
		return UndeclaredTX, fmt.Errorf("get tx type error,tx is nil ")
	}
	if blockchain.IsCoinBaseTx(tx) {
		return CoinbaseTX, nil
	}
	for _, txOut := range tx.TxOut {
		if txscript.IsPayToStakingScriptHash(txOut.PkScript) {
			return StakingTX, nil
		}
		if txscript.IsPayToBindingScriptHash(txOut.PkScript) {
			return BindingTX, nil
		}
	}
	for _, txIn := range tx.TxIn {
		hash := txIn.PreviousOutPoint.Hash
		index := txIn.PreviousOutPoint.Index
		tx, err := s.chain.GetTransaction(&hash)
		if err != nil {
			logging.CPrint(logging.ERROR, "No information available about transaction in db", logging.LogFormat{"err": err, "txId": hash.String()})
			st := status.New(ErrAPINoTxInfo, ErrCode[ErrAPINoTxInfo])
			return UndeclaredTX, st.Err()
		}
		if txscript.IsPayToStakingScriptHash(tx.TxOut[index].PkScript) {
			return StakingTX, nil
		}
		if txscript.IsPayToBindingScriptHash(tx.TxOut[index].PkScript) {
			return BindingTX, nil
		}
		if txscript.IsPayToPoolingScriptHash(tx.TxOut[index].PkScript) {
			return PoolingTX, nil
		}
	}
	return OrdinaryTX, nil
}

func createVoutList(mtx *wire.MsgTx, chainParams *config.Params, filterAddrMap map[string]struct{}) ([]*pb.Vout, int64, error) {
	if mtx == nil {
		logging.CPrint(logging.ERROR, "createVoutList error, mtx is nil ")
		return nil, 0, fmt.Errorf("createVoutList fail, mtx is nil ")
	}
	voutList := make([]*pb.Vout, 0, len(mtx.TxOut))
	var totalOutValue int64
	for i, v := range mtx.TxOut {
		// reset filter flag for each.
		passesFilter := len(filterAddrMap) == 0

		// The disassembled string will contain [error] inline if the
		// script doesn't fully parse, so ignore the error here.
		disbuf, err := txscript.DisasmString(v.PkScript)
		if err != nil {
			logging.CPrint(logging.WARN, "decode pkscript to asm exists err", logging.LogFormat{"err": err})
			st := status.New(ErrAPIDisasmScript, ErrCode[ErrAPIDisasmScript])
			return nil, -1, st.Err()
		}

		// Ignore the error here since an error means the script
		// couldn't parse and there is no additional information about
		// it anyways.
		scriptClass, addrs, _, reqSigs, err := txscript.ExtractPkScriptAddrs(
			v.PkScript, chainParams)
		if err != nil {
			st := status.New(ErrAPIExtractPKScript, ErrCode[ErrAPIExtractPKScript])
			return nil, -1, st.Err()
		}
		var frozenPeriod uint64
		var rewardAddress string
		if scriptClass == txscript.StakingScriptHashTy {
			_, pops := txscript.GetScriptInfo(v.PkScript)
			frozenPeriod, _, err = txscript.GetParsedOpcode(pops, scriptClass)
			if err != nil {
				return nil, -1, err
			}

			normalAddress, err := chainutil.NewAddressWitnessScriptHash(addrs[0].ScriptAddress(), chainParams)
			if err != nil {
				logging.CPrint(logging.ERROR, "createVoutList NewAddressWitnessScriptHash error", logging.LogFormat{"error": err})
				return nil, -1, err
			}
			rewardAddress = normalAddress.String()
		}

		encodedAddrs := make([]string, len(addrs))
		for j, addr := range addrs {
			encodedAddrs[j] = addr.EncodeAddress()

			if len(filterAddrMap) > 0 {
				if _, exists := filterAddrMap[encodedAddrs[j]]; exists {
					passesFilter = true
				}
			}
		}

		if !passesFilter {
			continue
		}

		totalOutValue += v.Value

		valueStr, err := AmountToString(v.Value)
		if err != nil {
			logging.CPrint(logging.ERROR, "createVoutList AmountToString error ")
			return nil, -1, err
		}

		vout := &pb.Vout{
			N:     uint32(i),
			Value: valueStr,
			ScriptPublicKey: &pb.ScriptPubKeyResult{
				Asm:           disbuf,
				Hex:           hex.EncodeToString(v.PkScript),
				ReqSigs:       uint32(reqSigs),
				Type:          scriptClass.String(),
				FrozenPeriod:  uint32(frozenPeriod),
				RewardAddress: rewardAddress,
				Addresses:     encodedAddrs,
			},
		}

		voutList = append(voutList, vout)
	}

	return voutList, totalOutValue, nil
}

func (s *Server) createVinList(mtx *wire.MsgTx, chainParams *config.Params) ([]*pb.Vin, []string, []*pb.InputsInTx, int64, error) {
	// Coinbase transactions only have a single txin by definition.
	vinList := make([]*pb.Vin, len(mtx.TxIn))
	addrs := make([]string, 0)
	inputs := make([]*pb.InputsInTx, 0)
	var totalInValue int64
	if blockchain.IsCoinBaseTx(mtx) {
		txIn := mtx.TxIn[0]
		vinTemp := &pb.Vin{
			TxId:         txIn.PreviousOutPoint.Hash.String(),
			Sequence:     txIn.Sequence,
			Witness:      txWitnessToHex(txIn.Witness),
			SequenceLock: &pb.SequenceLock{Seconds: 0, BlockHeight: 0},
		}
		vinList[0] = vinTemp

		for i, txIn := range mtx.TxIn[1:] {
			vinTemp := &pb.Vin{
				TxId:         txIn.PreviousOutPoint.Hash.String(),
				Vout:         txIn.PreviousOutPoint.Index,
				Sequence:     txIn.Sequence,
				SequenceLock: &pb.SequenceLock{Seconds: 0, BlockHeight: 0},
			}
			vinList[i+1] = vinTemp
		}

		return vinList, addrs, inputs, totalInValue, nil
	}
	tx := chainutil.NewTx(mtx)
	txStore := s.chain.GetTxPool().FetchInputTransactions(tx, false)
	sequenceLock, err := s.chain.CalcSequenceLock(tx, txStore)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	for i, txIn := range mtx.TxIn {
		vinEntry := &pb.Vin{
			TxId:     txIn.PreviousOutPoint.Hash.String(),
			Vout:     txIn.PreviousOutPoint.Index,
			Sequence: txIn.Sequence,
			Witness:  txWitnessToHex(txIn.Witness),
			SequenceLock: &pb.SequenceLock{
				Seconds:     sequenceLock.Seconds,
				BlockHeight: sequenceLock.BlockHeight,
			},
		}
		vinList[i] = vinEntry
		addrs, inValue, err := s.getTxInAddr(&txIn.PreviousOutPoint.Hash, txIn.PreviousOutPoint.Index, chainParams)
		if err != nil {
			logging.CPrint(logging.ERROR, "No information available about transaction in db", logging.LogFormat{"err": err.Error(), "txId": txIn.PreviousOutPoint.Hash.String()})
			st := status.New(ErrAPINoTxInfo, ErrCode[ErrAPINoTxInfo])
			return nil, nil, nil, -1, st.Err()
		}
		totalInValue = totalInValue + inValue
		val, err := AmountToString(inValue)
		if err != nil {
			logging.CPrint(logging.ERROR, "createVinList AmountToString")
			return nil, nil, nil, -1, err
		}
		inputs = append(inputs, &pb.InputsInTx{TxId: txIn.PreviousOutPoint.Hash.String(), Index: txIn.PreviousOutPoint.Index, Address: addrs, Value: val})
	}

	return vinList, addrs, inputs, totalInValue, nil
}

func (s *Server) getTxInAddr(txId *wire.Hash, index uint32, chainParams *config.Params) ([]string, int64, error) {
	addrStrs := make([]string, 0)
	var inValue int64
	tx, err := s.txMemPool.FetchTransaction(txId)
	var inmtx *wire.MsgTx
	if err != nil {
		txReply, err := s.chain.GetTransactionInDB(txId)
		if err != nil || len(txReply) == 0 {
			logging.CPrint(logging.ERROR, "No information available about transaction in db", logging.LogFormat{"err": err, "txId": txId.String()})
			st := status.New(ErrAPINoTxInfo, ErrCode[ErrAPINoTxInfo])
			return addrStrs, inValue, st.Err()
		}
		lastTx := txReply[len(txReply)-1]
		inmtx = lastTx.Tx
	} else {
		inmtx = tx.MsgTx()
	}

	_, addrs, _, _, err := txscript.ExtractPkScriptAddrs(inmtx.TxOut[int(index)].PkScript, chainParams)

	for _, addr := range addrs {
		addrStrs = append(addrStrs, addr.EncodeAddress())
	}
	inValue = inmtx.TxOut[int(index)].Value
	return addrStrs, inValue, nil
}

func (s *Server) getTxStatus(txHash *wire.Hash) (code int32, confirmations uint64, err error) {
	txList, err := s.chain.GetTransactionInDB(txHash)
	if err != nil || len(txList) == 0 {
		_, err := s.txMemPool.FetchTransaction(txHash)
		if err != nil {
			code = txStatusMissing
			return code, 0, nil
		} else {
			code = txStatusPacking
			//stats = "packing"
			return code, 0, nil
		}
	}

	lastTx := txList[len(txList)-1]
	txHeight := lastTx.Height
	bestHeight := s.chain.BestBlockHeight()
	confirmations = 1 + bestHeight - txHeight
	if blockchain.IsCoinBaseTx(lastTx.Tx) {
		if confirmations >= consensus.CoinbaseMaturity {
			code = txStatusConfirmed
			return code, confirmations, nil
		} else {
			code = txStatusConfirming
			return code, confirmations, nil
		}
	}
	if confirmations >= consensus.TransactionMaturity {
		code = txStatusConfirmed
		return code, confirmations, nil
	} else {
		code = txStatusConfirming
		return code, confirmations, nil
	}
}

func txWitnessToHex(witness wire.TxWitness) []string {
	// Ensure nil is returned when there are no entries versus an empty
	// slice so it can properly be omitted as necessary.
	if len(witness) == 0 {
		return nil
	}

	result := make([]string, 0, len(witness))
	for _, wit := range witness {
		result = append(result, hex.EncodeToString(wit))
	}

	return result
}
