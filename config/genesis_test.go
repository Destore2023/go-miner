package config

import (
	"github.com/Sukhavati-Labs/go-miner/wire"
	"testing"
)

func TestChainHeader(t *testing.T) {
	if len(genesisCoinbaseTx.TxOut) == 0 {
		t.Error("Chain Header Coinbase Tx out is empty!")
		t.FailNow()
	}
	//baseSubsidy := genesisCoinbaseTx.TxOut[0].Value
	//if uint64(baseSubsidy) != consensus.BaseSubsidy {
	//	t.Errorf("Chain Header Coinbase BaseSubsidy :%d , Consensus  BaseSubsidy:%d", baseSubsidy, consensus.BaseSubsidy)
	//	t.FailNow()
	//}
	blockHash := genesisHeader.BlockHash()
	t.Logf("Chain Header Block Hash:%s", blockHash)
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
	t.Logf("Chain Header ChainId:%s", chainId)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if !chainId.IsEqual(&genesisChainID) {
		t.Errorf("Chain Header Id Err")
		t.FailNow()
	}
	if !genesisHash.IsEqual(&blockHash) {
		t.Logf("Chain Header Block error!")
		t.FailNow()
	}
}

func TestGenesisDoc_Hash(t *testing.T) {
	if !ChainGenesisDoc.IsHashEqual(ChainGenesisDocHash) {
		t.Logf("genesis doc error!")
		t.FailNow()
	}
}
