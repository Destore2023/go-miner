package config

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
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
			Value:    0x2FAF08000,
			PkScript: mustDecodeString("002018ab40cde6f4f6087e27d118717bc80c176c5c4d16d2308423e893689b8d1823"),
		},
	},
	LockTime: 0,
	Payload:  mustDecodeString("000000000000000000000000"),
}

var genesisHeader = wire.BlockHeader{
	ChainID:         mustDecodeHash("8ecd463882c2ffcd9a82a6b85e52dcb3cb7e330819196334af7a9702ca2661ca"),
	Version:         1,
	Height:          0,
	Timestamp:       time.Unix(0x60531443, 0), // 2021-01-01 00:00:00 +0000 UTC, 1608250088 0x5FEE6600
	Previous:        mustDecodeHash("0000000000000000000000000000000000000000000000000000000000000000"),
	TransactionRoot: mustDecodeHash("ada8ce6758af6b5291349f58900f87f6b0e051fb43d9f756ea54c3fb708d950f"),
	WitnessRoot:     mustDecodeHash("ada8ce6758af6b5291349f58900f87f6b0e051fb43d9f756ea54c3fb708d950f"),
	ProposalRoot:    mustDecodeHash("9663440551fdcd6ada50b1fa1b0003d19bc7944955820b54ab569eb9a7ab7999"),
	Target:          hexToBigInt("44814ac47058"),
	Challenge:       mustDecodeHash("12166ef1d5772aff8fa94a7248d8d3601f8a3eb2faf4e46820b0f91b4d36a508"),
	PubKey:          mustDecodePoCPublicKey("033e44859a79ea399c69c41c248b4b33384813fe7b34735dd7d117acc08ae04d42"),
	Proof: &poc.Proof{
		X:         mustDecodeString("07149d0c"),
		XPrime:    mustDecodeString("941ea70c"),
		BitLength: 28,
	},
	Signature: mustDecodePoCSignature("3045022100dbcbafc7b41d4622394c55527aa43f582be7c8e7106579d26cf4b1a443cfd7e902203104a8ada1e64db2df7e88d0dafe548299cd878adddaf41b323c4a855c926c88"),
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

var genesisHash = mustDecodeHash("a03d8ac0d7899d5a92e38c47aa7f7e2cf493e9f0441bdeed887b43c65c1bf8eb")

var genesisChainID = mustDecodeHash("1b9d4594f1ede614b81a141d9b098ca6dc76a60f9efcf34d76c7c90645c4aa0b")

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

type GenesisProof struct {
	x         string `json:"x"`
	y         string `json:"y"`
	BitLength int    `json:"bit_length"`
}

type GenesisSignature struct {
	r string `json:"r"`
	s string `json:"s"`
}

type GenesisAlloc struct {
	Value   uint64 `json:"value"`
	Address string `json:"address"`
}

// GenesisDoc defines the initial conditions for a sukhavati blockchain, in particular its validator set.
type GenesisDoc struct {
	Version    uint64           `json: "version"`
	InitHeight uint64           `json:"init_height"`
	Timestamp  uint64           `json:"timestamp"`
	Target     string           `json:"target"`
	Challenge  string           `json:"challenge"`
	PublicKey  string           `json:"public_key"`
	Proof      GenesisProof     `json:"proof"`
	Signature  GenesisSignature `json:"signature"`
	alloc      []GenesisAlloc   `json:"alloc"`
	allocTxOut []*wire.TxOut
}

const ChainGenesisDocHash = "73756b686176617469b34ea2f85159fa271423fcc27496b5e2"

// SaveAs is a utility method for saving GenesisDoc as a JSON file.
func (genDoc GenesisDoc) SaveAs(file string) error {
	genDocBytes, err := json.Marshal(genDoc)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, genDocBytes, 0644)
}

func (genDoc GenesisDoc) IsHashEqual(sha string) bool {
	var data bytes.Buffer
	for _, v := range genDoc.allocTxOut {
		_, err := data.Write(v.PkScript)
		if err != nil {
			return false
		}
		var buf = make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(v.Value))
		_, err = data.Write(buf)
		if err != nil {
			return false
		}
	}
	hash := md5.New()
	hash.Write(data.Bytes())
	sum := hash.Sum([]byte("sukhavati"))
	//
	toString := hex.EncodeToString(sum)
	println("GenesisDocHash:" + toString)
	return bytes.Equal(sum, mustDecodeString(ChainGenesisDocHash))
}
