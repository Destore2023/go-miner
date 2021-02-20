package db

import (
	"encoding/hex"
	"errors"

	"github.com/Sukhavati-Labs/go-miner/poc"
	"github.com/Sukhavati-Labs/go-miner/poc/pocutil"
	"github.com/Sukhavati-Labs/go-miner/pocec"
)

type PocDB interface {
	// Get type of PocDB
	Type() string

	// Close PocDB
	Close() error

	// Execute plotting on PocDB
	Plot() chan error

	// Stop plotting on PocDB
	StopPlot() chan error

	// Is PocDB loaded and plotted
	Ready() bool

	// Get bitLength of PocDB
	BitLength() int

	// Get PubKeyHash of PocDB
	PubKeyHash() pocutil.Hash

	// Get PubKey of PocDB
	PubKey() *pocec.PublicKey

	// Get assembled proof by challenge
	GetProof(challenge pocutil.Hash) (proof *poc.Proof, err error)

	// Get plot progress
	Progress() (prePlotted, plotted bool, progress float64)

	// Delete all data of PocDB
	Delete() chan error
}

var (
	ErrInvalidDBType        = errors.New("invalid db type")
	ErrInvalidDBArgs        = errors.New("invalid db args")
	ErrDBDoesNotExist       = errors.New("non-existent db")
	ErrDBAlreadyExists      = errors.New("db already exists")
	ErrDBCorrupted          = errors.New("db corrupted")
	ErrUnimplemented        = errors.New("unimplemented db interface")
	ErrUnsupportedBitLength = errors.New("unsupported bit length")
)

// DoubleSHA256([]byte("POCDB"))
const DBFileCodeStr = "52A7AD74C4929DEC7B5C8D46CC3BAFA81FC96129283B3A6923CD12F41A30B3AC"

var (
	DBFileCode    []byte
	DBBackendList []DBBackend
)

type DBBackend struct {
	Typ      string
	OpenDB   func(args ...interface{}) (PocDB, error)
	CreateDB func(args ...interface{}) (PocDB, error)
}

func AddDBBackend(ins DBBackend) {
	for _, dbb := range DBBackendList {
		if dbb.Typ == ins.Typ {
			return
		}
	}
	DBBackendList = append(DBBackendList, ins)
}

func OpenDB(dbType string, args ...interface{}) (PocDB, error) {
	for _, dbb := range DBBackendList {
		if dbb.Typ == dbType {
			return dbb.OpenDB(args...)
		}
	}
	return nil, ErrInvalidDBType
}

func CreateDB(dbType string, args ...interface{}) (PocDB, error) {
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
