package config

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"time"

	"github.com/Sukhavati-Labs/go-miner/poc"

	"github.com/Sukhavati-Labs/go-miner/pocec"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

// genesisCoinbaseTx is the coinbase transaction for genesis block
var genesisCoinbaseTx = wire.MsgTx{
	Version: 1,
	TxIn: []*wire.TxIn{
		{
			PreviousOutPoint: wire.OutPoint{
				Hash:  wire.Hash{},
				Index: wire.MaxPrevOutIndex,
			},
			Sequence: wire.MaxTxInSequenceNum,
			Witness:  wire.TxWitness{},
		},
	},
	TxOut: []*wire.TxOut{
		{
			Value:    0x20EF7AC3840A00, //Investor sk1qqrz45pn0x7nmqsl386yv8z77gpstkchzdzmfrppprazfk3xudrq3samq95a
			PkScript: mustDecodeString("002018ab40cde6f4f6087e27d118717bc80c176c5c4d16d2308423e893689b8d1823"),
		},
		{
			Value:    0xD9D02F39EBB00, //Ecology sk1qqkla5a9a05pcfavtwnz8m2r296yzvlak26n7ksd7ev3q24j22qj9sr78w7w
			PkScript: mustDecodeString("0020b7fb4e97afa0709eb16e988fb50d45d104cff6cad4fd6837d96440aac94a048b"),
		},
		{
			Value:    0x15F4FC8454A700, //Team sk1qq2sv82wygyega90g3amckmtanxgcyaesn9r40zcqekq0yy7nyl6yqmkju74
			PkScript: mustDecodeString("002054187538882651d2bd11eef16dafb332304ee61328eaf16019b01e427a64fe88"),
		},
		{
			Value:    0xF5EB0C13E4B00, //Foundation sk1qq049qd3e48d2g0l520ldk4akut9vl7f4sr6a8pmfs57gth2swevkqghmmny
			PkScript: mustDecodeString("00207d4a06c7353b5487fe8a7fdb6af6dc5959ff26b01eba70ed30a790bbaa0ecb2c"),
		},
	},
	LockTime: 0,
	Payload:  mustDecodeString("000000000000000000000000"),
}

var genesisHeader = wire.BlockHeader{
	ChainID:         mustDecodeHash("8ecd463882c2ffcd9a82a6b85e52dcb3cb7e330819196334af7a9702ca2661ca"),
	Version:         1,
	Height:          0,
	Timestamp:       time.Unix(0x5FEE6600, 0), // 2021-01-01 00:00:00 +0000 UTC, 1608250088 0x5FEE6600
	Previous:        mustDecodeHash("0000000000000000000000000000000000000000000000000000000000000000"),
	TransactionRoot: mustDecodeHash("a4dc7ba67c6e75e87c0856071aaed9cb4e821f663b6bd84f37f6ac87dacdb676"),
	WitnessRoot:     mustDecodeHash("a4dc7ba67c6e75e87c0856071aaed9cb4e821f663b6bd84f37f6ac87dacdb676"),
	ProposalRoot:    mustDecodeHash("9663440551fdcd6ada50b1fa1b0003d19bc7944955820b54ab569eb9a7ab7999"),
	Target:          hexToBigInt("b5e620f48000"), // 200000000000000
	Challenge:       mustDecodeHash("5eb91b2d9fd6d5920ccc9610f0695509b60ccf764fab693ecab112f2edf1e3f0"),
	PubKey:          mustDecodePoCPublicKey("02c121b2bb27f8af5b365f1c0d9e02c2044a731aad6d0a6951ab3af506a3792c9c"),
	Proof: &poc.Proof{
		X:         mustDecodeString("acc59996"),
		XPrime:    mustDecodeString("944f0116"),
		BitLength: 32,
	},
	Signature: mustDecodePoCSignature("304402204ab4d572324785f59119a5dce949a47edb5b05cbf065e255510c23bcc9f0c133022027242ece09dee99ef19fa22d72a85a3db0662da1605300ae7610f03eab1d1a79"),
	BanList:   make([]*pocec.PublicKey, 0),
}

// genesisBlock defines the genesis block of the block chain which serves as the
// public transaction ledger.
var genesisBlock = wire.MsgBlock{
	Header: genesisHeader,
	Proposals: wire.ProposalArea{
		PunishmentArea: make([]*wire.FaultPubKey, 0),
		OtherArea:      make([]*wire.NormalProposal, 0),
	},
	Transactions: []*wire.MsgTx{&genesisCoinbaseTx},
}

var genesisHash = mustDecodeHash("b88c3b7bd49910541ea2e507dce3091ccc3b950c614c792a9cc9d7273d1600da")

var genesisChainID = mustDecodeHash("8ecd463882c2ffcd9a82a6b85e52dcb3cb7e330819196334af7a9702ca2661ca")

func hexToBigInt(str string) *big.Int {
	return new(big.Int).SetBytes(mustDecodeString(str))
}

func mustDecodeString(str string) []byte {
	buf, err := hex.DecodeString(str)
	if err != nil {
		panic(err)
	}
	return buf
}

func mustDecodeHash(str string) wire.Hash {
	h, err := wire.NewHashFromStr(str)
	if err != nil {
		panic(err)
	}
	return *h
}

func mustDecodePoCPublicKey(str string) *pocec.PublicKey {
	pub, err := pocec.ParsePubKey(mustDecodeString(str), pocec.S256())
	if err != nil {
		panic(err)
	}
	return pub
}

func mustDecodePoCSignature(str string) *pocec.Signature {
	sig, err := pocec.ParseSignature(mustDecodeString(str), pocec.S256())
	if err != nil {
		panic(err)
	}
	return sig
}

// GenesisDoc defines the initial conditions for a sukhavati blockchain, in particular its validator set.
type GenesisDoc struct {
	GenesisTime time.Time `json:"genesis_time"`
	ChainID     string    `json:"chain_id"`
}

// SaveAs is a utility method for saving GenesisDoc as a JSON file.
func (genDoc *GenesisDoc) SaveAs(file string) error {
	genDocBytes, err := json.Marshal(genDoc)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, genDocBytes, 0644)
}
