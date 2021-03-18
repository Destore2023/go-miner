package config

import (
	"encoding/hex"
	"github.com/Sukhavati-Labs/go-miner/consensus"
	"github.com/Sukhavati-Labs/go-miner/wire"
	"testing"
)

func TestChainHeader(t *testing.T) {
	if len(genesisCoinbaseTx.TxOut) == 0 {
		t.Error("Chain Header Coinbase Tx out is empty!")
		t.FailNow()
	}
	baseSubsidy := genesisCoinbaseTx.TxOut[0].Value
	if uint64(baseSubsidy) != consensus.BaseSubsidy {
		t.Errorf("Chain Header Coinbase BaseSubsidy :%d , Consensus  BaseSubsidy:%d", baseSubsidy, consensus.BaseSubsidy)
		t.FailNow()
	}
	blockHash := genesisHeader.BlockHash()
	t.Logf("Chain Header Block Hash:%s", blockHash)
	t.Logf("genesisHash:%s", genesisHash.String())
	t.Logf("%s", hex.EncodeToString(genesisHash[:]))
	for index, tx := range genesisBlock.Transactions {
		t.Logf("index:%d txSha:%s", index, tx.TxHash())
	}
	transactionMerkles := wire.BuildMerkleTreeStoreTransactions(genesisBlock.Transactions, false)
	size := len(transactionMerkles)
	transactionMerklesHash := transactionMerkles[size-1]
	t.Logf("Chain Header Block Transaction MerkleTree:%s", transactionMerklesHash)
	if !genesisHeader.TransactionRoot.IsEqual(transactionMerklesHash) {
		t.Errorf("Chain Header Block Transaction MerkleTree with error!")
		t.FailNow()
	}
	witnessMerkles := wire.BuildMerkleTreeStoreTransactions(genesisBlock.Transactions, true)
	size = len(witnessMerkles)
	witnessMerklesHash := witnessMerkles[size-1]
	t.Logf("Chain Header Block Transaction Witness Script MerkleTree:%s", transactionMerklesHash)
	if !genesisHeader.WitnessRoot.IsEqual(witnessMerklesHash) {
		t.Errorf("Chain Header Block Transaction Witness Script MerkleTree with error! %s\n", *witnessMerklesHash)
		t.FailNow()
	}
	proposalMerkles := wire.BuildMerkleTreeStoreForProposal(&genesisBlock.Proposals)
	size = len(proposalMerkles)
	proposalMerklesHash := proposalMerkles[size-1]
	if !genesisHeader.ProposalRoot.IsEqual(proposalMerklesHash) {
		t.Errorf("Chain Header Block Transaction Witness Script MerkleTree with error! %s\n", *witnessMerklesHash)
		t.FailNow()
	}
	chainId, err := genesisHeader.GetChainID()
	t.Logf("Chain Header ChainId:%s", hex.EncodeToString(chainId[:]))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !genesisChainID.IsEqual(&chainId) {
		t.Error("Chain Header Id error ")
		t.FailNow()
	}
	if !genesisHash.IsEqual(&blockHash) {
		t.Error("Chain Header Block error ")
		t.FailNow()
	}
}

func TestPocSignature(t *testing.T) {
	br, err := hex.DecodeString("dbcbafc7b41d4622394c55527aa43f582be7c8e7106579d26cf4b1a443cfd7e9")
	if err != nil {
		t.FailNow()
	}
	bs, err := hex.DecodeString("3104a8ada1e64db2df7e88d0dafe548299cd878adddaf41b323c4a855c926c88")
	if err != nil {
		t.FailNow()
	}
	sig := wire.NewEmptyPoCSignature()
	sig.R.SetBytes(br)
	sig.S.SetBytes(bs)
	signature := hex.EncodeToString(sig.Serialize())
	t.Logf("Signature:%s", signature)
}
