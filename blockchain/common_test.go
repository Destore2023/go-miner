package blockchain

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/consensus"
	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/database/ldb"
	"github.com/Sukhavati-Labs/go-miner/database/memdb"
	"github.com/Sukhavati-Labs/go-miner/errors"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/txscript"
	"github.com/Sukhavati-Labs/go-miner/wire"
	"github.com/btcsuite/btcd/btcec"
)

var (
	dataDir = "testdata"
	dbPath  = "./" + dataDir + "/testdb"
	logPath = "./" + dataDir + "/testlog"
	// dbType             = "leveldb"

	blocks200 []*chainutil.Block
)

// testDbRoot is the root directory used to create all test databases.
const testDbRoot = "testdbs"

// filesExists returns whether or not the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// Returns a new memory chaindb which is initialized with genesis block.
func newTestChainDb() (database.DB, error) {
	db, err := memdb.NewMemDb()
	if err != nil {
		return nil, err
	}

	// Get the latest block height from the database.
	_, height, err := db.NewestSha()
	if err != nil {
		db.Close()
		return nil, err
	}

	// Insert the appropriate genesis block for the Skt network being
	// connected to if needed.
	if height == math.MaxUint64 {
		block, err := loadNthBlock(1)
		if err != nil {
			return nil, err
		}
		err = db.InitByGenesisBlock(block)
		if err != nil {
			db.Close()
			return nil, err
		}
		height = block.Height()
		copy(config.ChainParams.GenesisHash[:], block.Hash()[:])
	}
	fmt.Printf("newTestChainDb initialized with block height %v\n", height)
	return db, nil
}

// loadTestBlocksIntoTestChainDb Loads fisrt N blocks (genesis not included) into db
func loadTestBlocksIntoTestChainDb(db database.DB, numBlocks int) error {
	blocks, err := loadTopNBlock(numBlocks)
	if err != nil {
		return err
	}

	for i, block := range blocks {
		if i == 0 {
			continue
		}

		err = db.SubmitBlock(block, nil)
		if err != nil {
			return err
		}
		cdb := db.(*ldb.ChainDb)
		cdb.Batch(1).Set(*block.Hash())
		cdb.Batch(1).Done()
		err = db.Commit(*block.Hash())
		if err != nil {
			return err
		}

		if i == numBlocks-1 {
			// // used to print tx info
			// txs := block.Transactions()
			// tx1 := txs[1]
			// for i, txIn := range tx1.MsgTx().TxIn {
			// 	h := txIn.PreviousOutPoint.Hash
			// 	fmt.Println(i, h, h[:], txIn.PreviousOutPoint.Index)
			// 	fmt.Println(txIn.Sequence)
			// 	fmt.Println(txIn.Witness)
			// }
			// fmt.Println(tx1.MsgTx().LockTime, tx1.MsgTx().Payload)
			// for i, txOut := range tx1.MsgTx().TxOut {
			// 	fmt.Println(i, txOut.Value, txOut.PkScript)
			// }
			break
		}
	}

	_, height, err := db.NewestSha()
	if err != nil {
		return err
	}
	if height != uint64(numBlocks-1) {
		return errors.New("incorrect best height")
	}
	return nil
}

func newTestBlockchain(db database.DB, blockCachePath string) (*Blockchain, error) {
	chain, err := NewBlockchain(db, blockCachePath, nil)
	if err != nil {
		return nil, err
	}
	chain.GetTxPool().SetNewTxCh(make(chan *chainutil.Tx, 2000)) // prevent deadlock
	return chain, nil
}

func init() {
	f, err := os.Open("./data/mockBlks.dat")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		buf, err := hex.DecodeString(scanner.Text())
		if err != nil {
			panic(err)
		}

		block, err := chainutil.NewBlockFromBytes(buf, wire.Packet)
		if err != nil {
			panic(err)
		}
		blocks200 = append(blocks200, block)
	}
}

// the 1st block is genesis
func loadNthBlock(nth int) (*chainutil.Block, error) {
	if nth <= 0 || nth > len(blocks200) {
		return nil, errors.New("invalid nth")
	}
	return blocks200[nth-1], nil
}

// the 1st block is genesis
func loadTopNBlock(n int) ([]*chainutil.Block, error) {
	if n <= 0 || n > len(blocks200) {
		return nil, errors.New("invalid n")
	}

	return blocks200[:n], nil
}

func newWitnessScriptAddress(pubkeys []*btcec.PublicKey, nrequired int,
	addressClass uint16, net *config.Params) ([]byte, chainutil.Address, error) {

	var addressPubKeyStructs []*chainutil.AddressPubKey
	for i := 0; i < len(pubkeys); i++ {
		pubKeySerial := pubkeys[i].SerializeCompressed()
		addressPubKeyStruct, err := chainutil.NewAddressPubKey(pubKeySerial, net)
		if err != nil {
			logging.CPrint(logging.ERROR, "create addressPubKey failed",
				logging.LogFormat{
					"err":       err,
					"version":   addressClass,
					"nrequired": nrequired,
				})
			return nil, nil, err
		}
		addressPubKeyStructs = append(addressPubKeyStructs, addressPubKeyStruct)
	}

	redeemScript, err := txscript.MultiSigScript(addressPubKeyStructs, nrequired)
	if err != nil {
		logging.CPrint(logging.ERROR, "create redeemScript failed",
			logging.LogFormat{
				"err":       err,
				"version":   addressClass,
				"nrequired": nrequired,
			})
		return nil, nil, err
	}
	var witAddress chainutil.Address
	// scriptHash is witnessProgram
	scriptHash := sha256.Sum256(redeemScript)
	switch addressClass {
	case chainutil.AddressClassWitnessStaking:
		witAddress, err = chainutil.NewAddressStakingScriptHash(scriptHash[:], net)
	case chainutil.AddressClassWitnessV0:
		witAddress, err = chainutil.NewAddressWitnessScriptHash(scriptHash[:], net)
	default:
		return nil, nil, errors.New("invalid version")
	}

	if err != nil {
		logging.CPrint(logging.ERROR, "create witness address failed",
			logging.LogFormat{
				"err":       err,
				"version":   addressClass,
				"nrequired": nrequired,
			})
		return nil, nil, err
	}

	return redeemScript, witAddress, nil
}

func mkTmpDir(dir string) (func(), error) {
	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(dir, 0700)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return func() {
		os.RemoveAll(dir)
	}, nil
}

var (
	comTestCoinbaseMaturity     uint64 = 20
	comTestMinStakingValue             = 100 * consensus.SukhavatiPerSkt
	comTestMinFrozenPeriod      uint64 = 4
	comTestStakingTxRewardStart uint64 = 2
)

// make sure to be consistent with mocked blocks
func TestMain(m *testing.M) {
	consensus.CoinbaseMaturity = comTestCoinbaseMaturity
	consensus.MinStakingValue = comTestMinStakingValue
	consensus.MinFrozenPeriod = comTestMinFrozenPeriod
	consensus.StakingTxRewardStart = comTestStakingTxRewardStart

	fmt.Println("coinbase maturity:", consensus.CoinbaseMaturity)
	fmt.Println("mininum frozen period:", consensus.MinFrozenPeriod)
	fmt.Println("mininum staking value:", consensus.MinStakingValue)

	os.Exit(m.Run())
}
