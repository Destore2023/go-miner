package sktdb

import (
	"encoding/hex"
	"errors"

	"github.com/Sukhavati-Labs/go-miner/poc"
	"github.com/Sukhavati-Labs/go-miner/poc/pocutil"
	"github.com/Sukhavati-Labs/go-miner/pocec"
)

type SktDB interface {
	// Get type of SktDB
	Type() string

	// Close SktDB
	Close() error

	// Execute plotting on SktDB
	Plot() chan error

	// Stop plotting on SktDB
	StopPlot() chan error

	// Is SktDB loaded and plotted
	Ready() bool

	// Get bitLength of SktDB
	BitLength() int

	// Get PubKeyHash of SktDB
	PubKeyHash() pocutil.Hash

	// Get PubKey of SktDB
	PubKey() *pocec.PublicKey

	// Get assembled proof by challenge
	GetProof(challenge pocutil.Hash) (proof *poc.Proof, err error)

	// Get plot progress
	Progress() (prePlotted, plotted bool, progress float64)

	// Delete all data of SktDB
	Delete() chan error
}

var (
	ErrInvalidDBType        = errors.New("invalid sktdb type")
	ErrInvalidDBArgs        = errors.New("invalid sktdb args")
	ErrDBDoesNotExist       = errors.New("non-existent sktdb")
	ErrDBAlreadyExists      = errors.New("sktdb already exists")
	ErrDBCorrupted          = errors.New("sktdb corrupted")
	ErrUnimplemented        = errors.New("unimplemented sktdb interface")
	ErrUnsupportedBitLength = errors.New("unsupported bit length")
)

// TODO compatible with multiple db prefix
// DoubleSHA256([]byte("MASSDB"))
const DBFileCodeStr = "52A7AD74C4929DEC7B5C8D46CC3BAFA81FC96129283B3A6923CD12F41A30B3AC"

var (
	DBFileCode    []byte
	DBBackendList []DBBackend
)

type DBBackend struct {
	Typ      string
	OpenDB   func(args ...interface{}) (SktDB, error)
	CreateDB func(args ...interface{}) (SktDB, error)
}

func AddDBBackend(ins DBBackend) {
	for _, dbb := range DBBackendList {
		if dbb.Typ == ins.Typ {
			return
		}
	}
	DBBackendList = append(DBBackendList, ins)
}

func OpenDB(dbType string, args ...interface{}) (SktDB, error) {
	for _, dbb := range DBBackendList {
		if dbb.Typ == dbType {
			return dbb.OpenDB(args...)
		}
	}
	return nil, ErrInvalidDBType
}

func CreateDB(dbType string, args ...interface{}) (SktDB, error) {
	for _, dbb := range DBBackendList {
		if dbb.Typ == dbType {
			return dbb.CreateDB(args...)
		}
	}
	return nil, ErrInvalidDBType
}

func init() {
	var err error
	DBFileCode, err = hex.DecodeString(DBFileCodeStr)
	if err != nil || len(DBFileCode) != 32 {
		panic(err) // should never happen
	}
}
