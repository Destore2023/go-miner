package blockchain

import (
	"container/heap"
	"container/list"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/chainutil/safetype"
	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/consensus"
	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/poc"
	"github.com/Sukhavati-Labs/go-miner/pocec"
	"github.com/Sukhavati-Labs/go-miner/txscript"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

const (
	// generatedBlockVersion is the version of the block being generated.
	// It is defined as a constant here rather than using the
	// wire.BlockVersion constant since a change in the block version
	// will require changes to the generated block.  Using the wire constant
	// for generated block version could allow creation of invalid blocks
	// for the updated version.
	generatedBlockVersion = wire.BlockVersion

	// blockHeaderOverhead is the min number of bytes it takes to serialize
	// a block header.
	blockHeaderOverhead = wire.MinBlockHeaderPayload

	// maximum number of binding evidence(recommended)
	MaxBindingNum = 10

	// 36 prev outpoint, 8 sequence
	bindingPayload = 44 * MaxBindingNum

	// minHighPriority is the minimum priority value that allows a
	// transaction to be considered high priority.
	minHighPriority = consensus.MinHighPriority

	PriorityProposalSize = wire.MaxBlockPayload / 20
)

var (
	anyoneRedeemableScript  []byte
	stakingPoolPubKeyScript []byte
)

func init() {
	var err error
	anyoneRedeemableScript, err = txscript.NewScriptBuilder().AddOp(txscript.OP_TRUE).Script()
	if err != nil {
		panic("init anyoneRedeemableScript: " + err.Error())
	}
	address, err := chainutil.NewAddressPoolingScriptHash([]byte(consensus.StakingPoolAddressScriptHash), &config.ChainParams)
	if err != nil {
		panic("init poolPubKeyScript: " + err.Error())
	}
	stakingPoolPubKeyScript, err = txscript.PayToPoolingAddrScript(address, consensus.StakingPoolType)
	if err != nil {
		panic("init poolPubKeyScript: " + err.Error())
	}
}

// txPrioItem houses a transaction along with extra information that allows the
// transaction to be prioritized and track dependencies on other transactions
// which have not been mined into a block yet.
// Priority --> Prio
type txPrioItem struct {
	tx       *chainutil.Tx
	fee      int64
	priority float64
	feePerKB float64

	// dependsOn holds a map of transaction hashes which this one depends
	// on.  It will only be set when the transaction references other
	// transactions in the memory pool and hence must come after them in
	// a block.
	dependsOn map[wire.Hash]struct{}
}

// txPriorityQueueLessFunc describes a function that can be used as a compare
// function for a transaction priority queue (txPriorityQueue).
type txPriorityQueueLessFunc func(*txPriorityQueue, int, int) bool

// txPriorityQueue implements a priority queue of txPrioItem elements that
// supports an arbitrary compare function as defined by txPriorityQueueLessFunc.
type txPriorityQueue struct {
	lessFunc txPriorityQueueLessFunc
	items    []*txPrioItem
}

// CoinbasePayload BIP34
// fix duplicate coinbase transactions potentially causing two coinbase transactions becoming invalid
type CoinbasePayload struct {
	height           uint64
	numStakingReward uint32
}

func (p *CoinbasePayload) NumStakingReward() uint32 {
	return p.numStakingReward
}

func (p *CoinbasePayload) Bytes() []byte {
	buf := make([]byte, 12)
	binary.LittleEndian.PutUint64(buf[:8], p.height)
	binary.LittleEndian.PutUint32(buf[8:12], p.numStakingReward)
	return buf
}

func (p *CoinbasePayload) SetBytes(data []byte) error {
	if len(data) < 12 {
		return errIncompleteCoinbasePayload
	}
	p.height = binary.LittleEndian.Uint64(data[0:8])
	p.numStakingReward = binary.LittleEndian.Uint32(data[8:12])
	return nil
}

func NewCoinbasePayload() *CoinbasePayload {
	return &CoinbasePayload{
		height:           0,
		numStakingReward: 0,
	}
}

func standardCoinbasePayload(nextBlockHeight uint64, numStakingReward uint32) []byte {
	p := &CoinbasePayload{
		height:           nextBlockHeight,
		numStakingReward: numStakingReward,
	}
	return p.Bytes()
}

// Len returns the number of items in the priority queue.  It is part of the
// heap.Interface implementation.
func (pq *txPriorityQueue) Len() int {
	return len(pq.items)
}

// Less returns whether the item in the priority queue with index i should sort
// before the item with index j by deferring to the assigned less function.  It
// is part of the heap.Interface implementation.
func (pq *txPriorityQueue) Less(i, j int) bool {
	return pq.lessFunc(pq, i, j)
}

// Swap swaps the items at the passed indices in the priority queue.  It is
// part of the heap.Interface implementation.
func (pq *txPriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
}

// Push pushes the passed item onto the priority queue.  It is part of the
// heap.Interface implementation.
func (pq *txPriorityQueue) Push(x interface{}) {
	pq.items = append(pq.items, x.(*txPrioItem))
}

// Pop removes the highest priority item (according to Less) from the priority
// queue and returns it.  It is part of the heap.Interface implementation.
func (pq *txPriorityQueue) Pop() interface{} {
	n := len(pq.items)
	item := pq.items[n-1]
	pq.items[n-1] = nil
	pq.items = pq.items[0 : n-1]
	return item
}

// SetLessFunc sets the compare function for the priority queue to the provided
// function.  It also invokes heap.Init on the priority queue using the new
// function so it can immediately be used with heap.Push/Pop.
func (pq *txPriorityQueue) SetLessFunc(lessFunc txPriorityQueueLessFunc) {
	pq.lessFunc = lessFunc
	heap.Init(pq)
}

// txPQByPriority sorts a txPriorityQueue by transaction priority and then fees
// per kilobyte.
func txPQByPriority(pq *txPriorityQueue, i, j int) bool {
	// Using > here so that pop gives the highest priority item as opposed
	// to the lowest.  Sort by priority first, then fee.
	if pq.items[i].priority == pq.items[j].priority {
		return pq.items[i].feePerKB > pq.items[j].feePerKB
	}
	return pq.items[i].priority > pq.items[j].priority

}

// txPQByFee sorts a txPriorityQueue by fees per kilobyte and then transaction
// priority.
func txPQByFee(pq *txPriorityQueue, i, j int) bool {
	// Using > here so that pop gives the highest fee item as opposed
	// to the lowest.  Sort by fee first, then priority.
	if pq.items[i].feePerKB == pq.items[j].feePerKB {
		return pq.items[i].priority > pq.items[j].priority
	}
	return pq.items[i].feePerKB > pq.items[j].feePerKB
}

// newTxPriorityQueue returns a new transaction priority queue that reserves the
// passed amount of space for the elements.  The new priority queue uses either
// the txPQByPriority or the txPQByFee compare function depending on the
// sortByFee parameter and is already initialized for use with heap.Push/Pop.
// The priority queue can grow larger than the reserved space, but extra copies
// of the underlying array can be avoided by reserving a sane value.
func newTxPriorityQueue(reserve int, sortByFee bool) *txPriorityQueue {
	pq := &txPriorityQueue{
		items: make([]*txPrioItem, 0, reserve),
	}
	if sortByFee {
		pq.SetLessFunc(txPQByFee)
	} else {
		pq.SetLessFunc(txPQByPriority)
	}
	return pq
}

type PoCTemplate struct {
	Height        uint64
	Timestamp     time.Time
	Previous      wire.Hash
	Challenge     wire.Hash
	GetTarget     func(time.Time) *big.Int
	RewardAddress []database.Rank
	GetCoinbase   func(*pocec.PublicKey, chainutil.Amount, int) (*chainutil.Tx, error)
	Err           error
}

type BlockTemplate struct {
	Block              *wire.MsgBlock   // block
	TotalFee           chainutil.Amount // total fee  |  fee + burn gas
	BurnGas            chainutil.Amount // burn gas
	SigOpCounts        []int64
	Height             uint64
	ValidPayAddress    bool
	MerkleCache        []*wire.Hash
	WitnessMerkleCache []*wire.Hash
	Err                error
}

// Miner Reward Tx Out
// +--------------+
// | staking pool |
// +--------------+
// | senate Reward|
// +--------------+
// | Miner Reward |
// +--------------+
func GetMinerRewardTxOutFromCoinbase(coinbase *wire.MsgTx) (*wire.TxOut, error) {
	if !coinbase.IsCoinBaseTx() {
		return nil, ErrCoinbaseTx
	}
	l := len(coinbase.TxOut)
	if l == 0 {
		return nil, ErrCoinbaseTx
	}
	minerTxOut := coinbase.TxOut[l-1]
	return minerTxOut, nil
}

func mustDecodeString(str string) []byte {
	buf, err := hex.DecodeString(str)
	if err != nil {
		panic(err)
	}
	return buf
}

func createDistributeTx(coinbase *wire.MsgTx, nextBlockHeight uint64) error {
	if nextBlockHeight == config.ChainGenesisDoc.InitHeight {
		for _, c := range config.ChainGenesisDoc.AllocTxOut {
			coinbase.AddTxOut(c)
		}
	}
	return nil
}

//  Coinbase Tx
//   Vin:                           Vout:
//  +------------------------------+-----------------------------------+
//  | coinbase Genesis in          |   staking pool Genesis out        |
//  |                              |-----------------------------------|
//  |                              |   senate Genesis out              |
//  |                              |-----------------------------------|
//  |                              |   miner Genesis out               |
//  +------------------------------------------------------------------+
func reCreateCoinbaseTx(coinbase *wire.MsgTx, bindingTxListReply []*database.BindingTxReply, nextBlockHeight uint64,
	bitLength int, rewardAddresses []database.Rank, senateEquities []*database.SenateWeight, totalFee chainutil.Amount) (err error) {
	minerTxOut, err := GetMinerRewardTxOutFromCoinbase(coinbase)
	if err != nil {
		logging.CPrint(logging.ERROR, "reCreateCoinbaseTx get miner tx out ", logging.LogFormat{"error": err, "height": nextBlockHeight})
		return err
	}
	coinbase.RemoveAllTxOut()
	err = createDistributeTx(coinbase, nextBlockHeight)
	if err != nil {
		logging.CPrint(logging.ERROR, "reCreateCoinbaseTx createDistributeTx", logging.LogFormat{"error": err, "height": nextBlockHeight})
		return err
	}
	totalBinding := chainutil.ZeroAmount()
	// means still have reward in coinbase
	// originMiner cannot be smaller than diff
	// has guaranty tx
	txIns := make([]*wire.TxIn, 0)
	hasValidBinding := false
	if len(bindingTxListReply) > 0 {
		valueRequired, ok := bindingRequiredAmount[bitLength]
		// valid bit length
		if ok {
			var witness [][]byte
			bindingNum := 0
			for _, bindingTx := range bindingTxListReply {
				txHash := bindingTx.TxSha
				index := bindingTx.Index
				blocksSincePrev := nextBlockHeight - bindingTx.Height
				if bindingTx.IsCoinbase {
					if blocksSincePrev < consensus.CoinbaseMaturity {
						logging.CPrint(logging.WARN, "the txIn is not mature", logging.LogFormat{"txId": txHash.String(), "index": index})
						continue
					}
				} else {
					if blocksSincePrev < consensus.TransactionMaturity {
						logging.CPrint(logging.WARN, "the txIn is not mature", logging.LogFormat{"txId": txHash.String(), "index": index})
						continue
					}
				}
				totalBinding, err = totalBinding.AddInt(bindingTx.Value)
				if err != nil {
					logging.CPrint(logging.ERROR, "reCreateCoinbaseTx totalBinding", logging.LogFormat{"error": err, "height": nextBlockHeight})
					return err
				}
				prevOut := wire.NewOutPoint(txHash, index)
				txIn := wire.NewTxIn(prevOut, witness)
				txIns = append(txIns, txIn)
				if totalBinding.Cmp(valueRequired) >= 0 {
					hasValidBinding = true
					break
				}
				bindingNum++
				if bindingNum >= MaxBindingNum {
					break
				}
			}
		} else {
			if bitLength != bitLengthMissing {
				logging.CPrint(logging.DEBUG, "invalid bit length",
					logging.LogFormat{"bitLength": bitLength})
			}
		}
	} else {
		logging.CPrint(logging.INFO, "No binding tx in the pubkey")
	}

	miner, poolNode, senateNode, err := CalcBlockSubsidy(nextBlockHeight, &config.ChainParams, totalBinding, bitLength)
	if err != nil {
		logging.CPrint(logging.ERROR, "fail on CalcBlockSubsidy", logging.LogFormat{
			"err":                    err,
			"height":                 nextBlockHeight,
			"total_binding":          totalBinding,
			"reward_addresses_count": len(rewardAddresses),
			"bit_length":             bitLength,
		})
		return err
	}
	// pool node
	if !poolNode.IsZero() {
		coinbase.AddTxOut(&wire.TxOut{
			Value:    poolNode.IntValue(),
			PkScript: stakingPoolPubKeyScript,
		})
	}
	// mint , there is no miner output
	if miner.IsZero() {
		miner, err = miner.Add(totalFee)
		if err != nil {
			logging.CPrint(logging.ERROR, "reCreateCoinbaseTx miner reward",
				logging.LogFormat{
					"error":    err,
					"height":   nextBlockHeight,
					"totalFee": totalFee})
			return err
		}
		coinbase.AddTxOut(&wire.TxOut{
			PkScript: minerTxOut.PkScript,
			Value:    miner.IntValue(),
		})
		return
	}
	// restore totalBinding as coinbase
	if hasValidBinding {
		for _, txIn := range txIns {
			coinbase.AddTxIn(txIn)
		}
	}

	logging.CPrint(logging.INFO, "show the count of stakingTx", logging.LogFormat{"count": len(rewardAddresses)})
	if senateNode.IsZero() {
		err := fmt.Errorf("reCreateCoinbaseTx senateNode reward is 0")
		logging.CPrint(logging.ERROR, "reCreateCoinbaseTx senateNode reward is 0",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		return err
	}
	if len(senateEquities) == 0 {
		err := fmt.Errorf("reCreateCoinbaseTx senateEquities is empty")
		logging.CPrint(logging.ERROR, "reCreateCoinbaseTx senateEquities is empty",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		return err
	}
	// senateNode
	var totalSenateValue uint64 = 0
	for _, equity := range senateEquities {
		totalSenateValue = equity.Weight + totalSenateValue
	}
	for _, equity := range senateEquities {
		pkScriptSenateNode, err := txscript.PayToWitnessScriptHashScript(equity.ScriptHash[:])
		if err != nil {
			logging.CPrint(logging.ERROR, "reCreateCoinbaseTx senateNode reward pkScriptSenateNode",
				logging.LogFormat{
					"error":  err,
					"height": nextBlockHeight})
			return err
		}
		senateReward, err := senateNode.Value().MulInt(int64(equity.Weight))
		if err != nil {
			logging.CPrint(logging.ERROR, "reCreateCoinbaseTx senateNode reward senateReward MulInt",
				logging.LogFormat{
					"error":  err,
					"height": nextBlockHeight})
			return err
		}
		senateReward, err = senateReward.DivInt(int64(totalSenateValue))
		if err != nil {
			logging.CPrint(logging.ERROR, "reCreateCoinbaseTx senateNode reward senateReward DivInt",
				logging.LogFormat{
					"error":  err,
					"height": nextBlockHeight})
			return err
		}
		coinbase.AddTxOut(&wire.TxOut{
			Value:    senateReward.IntValue(),
			PkScript: pkScriptSenateNode,
		})
	}
	miner, err = miner.Add(totalFee)
	if err != nil {
		logging.CPrint(logging.ERROR, "reCreateCoinbaseTx total miner",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		return err
	}
	// miner as first out  and update value later
	coinbase.AddTxOut(&wire.TxOut{
		PkScript: minerTxOut.PkScript,
		Value:    miner.IntValue(),
	})
	coinbase.SetPayload(standardCoinbasePayload(nextBlockHeight, 0))
	return
}

// createStakingPoolMergeTx
// +-------------------------------+----------------------------------+
// | staking pool reward           |                                  |
// +-------------------------------+                                  |
// | staking pool reward           | staking pool reward              |
// +-------------------------------+                                  |
// | staking pool reward           |                                  |
// +-------------------------------+----------------------------------+
func createStakingPoolMergeTx(nextBlockHeight uint64, unspentPoolTxs []*database.TxReply) (*chainutil.Tx, error) {
	stakingPoolMergeTx := wire.NewMsgTx()
	var poolWitness [][]byte
	poolMaturityValue := safetype.NewUint128()
	var err error
	for _, reply := range unspentPoolTxs {
		blocksSincePrev := nextBlockHeight - reply.Height
		isCoinbase := reply.Tx.IsCoinBaseTx()
		for index, txOut := range reply.Tx.TxOut {
			if reply.TxSpent[index] {
				continue
			}
			if !txscript.IsPayToPoolingScriptHash(txOut.PkScript) {
				continue
			}
			if blocksSincePrev < consensus.TransactionMaturity {
				continue
			}
			if isCoinbase && blocksSincePrev < consensus.CoinbaseMaturity {
				continue
			}
			poolMaturityValue, err = poolMaturityValue.AddInt(txOut.Value)
			if err != nil {
				logging.CPrint(logging.ERROR, "createStakingPoolMergeTx  poolMaturityValue",
					logging.LogFormat{
						"error":  err,
						"height": nextBlockHeight})
				return nil, err
			}
			prevOut := wire.NewOutPoint(reply.TxSha, uint32(index))
			//poolWitness, err = txscript.SignTxOutputWit(&config.ChainParams)
			poolTxIn := wire.NewTxIn(prevOut, poolWitness)
			stakingPoolMergeTx.AddTxIn(poolTxIn)
		}
	}
	if len(stakingPoolMergeTx.TxIn) == 0 {
		return nil, err
	}
	if !poolMaturityValue.IsZero() {
		stakingPoolMergeTx.AddTxOut(&wire.TxOut{
			Value:    poolMaturityValue.IntValue(),
			PkScript: stakingPoolPubKeyScript,
		})
	}
	return chainutil.NewTx(stakingPoolMergeTx), nil
}

// createStakingPoolRewardTx
// +-------------------------------+-----------------------------------+
// | staking pool reward           |  staking pool accumulated out     |
// |                               |-----------------------------------+
// |                               |  reward                           |
// +-------------------------------+-----------------------------------+
func createStakingPoolRewardTx(nextBlockHeight uint64, rewardAddresses []database.Rank, unspentPoolTxs []*database.TxReply) (*chainutil.Tx, error) {
	stakingPoolRewardTx := wire.NewMsgTx()
	var poolWitness [][]byte
	poolTotal := safetype.NewUint128()
	poolMaturityValue := safetype.NewUint128()
	poolReward := chainutil.ZeroAmount()
	var err error
	for _, reply := range unspentPoolTxs {
		blocksSincePrev := nextBlockHeight - reply.Height
		isCoinbase := reply.Tx.IsCoinBaseTx()
		for index, txOut := range reply.Tx.TxOut {
			if reply.TxSpent[index] {
				continue
			}
			if !txscript.IsPayToPoolingScriptHash(txOut.PkScript) {
				continue
			}
			poolTotal, err = poolTotal.AddInt(txOut.Value)
			if err != nil {
				logging.CPrint(logging.ERROR, "createStakingPoolRewardTx  poolTotal",
					logging.LogFormat{
						"error":  err,
						"height": nextBlockHeight})
				return nil, err
			}
			if blocksSincePrev < consensus.TransactionMaturity {
				continue
			}
			if isCoinbase && blocksSincePrev < consensus.CoinbaseMaturity {
				continue
			}
			poolMaturityValue, err = poolMaturityValue.AddInt(txOut.Value)
			if err != nil {
				logging.CPrint(logging.ERROR, "createStakingPoolRewardTx  poolMaturityValue",
					logging.LogFormat{
						"error":  err,
						"height": nextBlockHeight})
				return nil, err
			}
			prevOut := wire.NewOutPoint(reply.TxSha, uint32(index))
			poolTxIn := wire.NewTxIn(prevOut, poolWitness)
			stakingPoolRewardTx.AddTxIn(poolTxIn)
		}
	}
	//  1/200 pool reward
	//if coinbasePayload.LastStakingAwardedTimestamp() == 0 || (uint64(time.Now().Unix())-coinbasePayload.LastStakingAwardedTimestamp()) < 86400 {
	divInt, err := poolTotal.DivInt(consensus.StakingPoolRewardProportionalDenominator)
	if err != nil {
		logging.CPrint(logging.ERROR, "createStakingPoolRewardTx  poolTotal div ",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		return nil, err
	}

	if divInt.Gt(poolMaturityValue) {
		poolReward, err = poolReward.AddInt(poolMaturityValue.IntValue())
		if err != nil {
			logging.CPrint(logging.ERROR, "createStakingPoolRewardTx  poolTotal div >  poolMaturityValue",
				logging.LogFormat{
					"error":  err,
					"height": nextBlockHeight})
			return nil, err
		}
	} else {
		poolReward, err = poolReward.AddInt(divInt.IntValue())
	}
	if err != nil {
		logging.CPrint(logging.ERROR, "createStakingPoolRewardTx  poolReward",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		return nil, err
	}
	totalWeight := safetype.NewUint128()
	for _, v := range rewardAddresses {
		totalWeight, err = totalWeight.Add(v.Weight)
		if err != nil {
			logging.CPrint(logging.ERROR, "createStakingPoolRewardTx  totalWeight",
				logging.LogFormat{
					"error":  err,
					"height": nextBlockHeight})
			return nil, err
		}
	}
	poolBalance := safetype.NewUint128FromUint(poolReward.UintValue())
	// calc reward  index start with 0
	var numOfStakingRewardSent uint32 = 0
	for i := 0; i < len(rewardAddresses); i++ {
		key := make([]byte, sha256.Size)
		copy(key, rewardAddresses[i].ScriptHash[:])
		pkScriptSuperNode, err := txscript.PayToWitnessScriptHashScript(key)
		if err != nil {
			logging.CPrint(logging.ERROR, "createStakingPoolRewardTx  PayToWitnessScriptHashScript",
				logging.LogFormat{
					"error":  err,
					"pubkey": key,
					"height": nextBlockHeight})
			return nil, err
		}

		nodeWeight := rewardAddresses[i].Weight
		nodeReward, err := calcNodeReward(poolReward, totalWeight, nodeWeight)
		if err != nil {
			logging.CPrint(logging.ERROR, "createStakingPoolRewardTx  calcNodeReward",
				logging.LogFormat{
					"error":  err,
					"height": nextBlockHeight})
			return nil, err
		}
		logging.CPrint(logging.INFO, "createStakingPoolRewardTx reward",
			logging.LogFormat{
				"script hash":  key,
				"node weight":  nodeWeight,
				"total weight": totalWeight,
				"node reward":  nodeReward,
				"pool Reward":  poolReward,
			})
		if nodeReward.IsZero() {
			// break loop as rewordAddress is in descending order by value
			break
		}
		numOfStakingRewardSent = numOfStakingRewardSent + 1
		stakingPoolRewardTx.AddTxOut(&wire.TxOut{
			Value:    nodeReward.IntValue(),
			PkScript: pkScriptSuperNode,
		})
		poolBalance, err = poolBalance.SubUint(nodeReward.UintValue())
		if err != nil {
			logging.CPrint(logging.ERROR, "createStakingPoolRewardTx  poolBalance",
				logging.LogFormat{
					"error":       err,
					"height":      nextBlockHeight,
					"poolBalance": poolBalance,
				})
			return nil, err
		}
	}

	// poolNode balance
	if !poolBalance.IsZero() {
		stakingPoolRewardTx.AddTxOut(&wire.TxOut{
			Value:    poolBalance.IntValue(),
			PkScript: stakingPoolPubKeyScript,
		})
	}
	return chainutil.NewTx(stakingPoolRewardTx), nil
}

// createCoinbaseTx returns a coinbase transaction paying an appropriate subsidy
// based on the passed block height to the provided address.  When the address
// is nil, the coinbase transaction will instead be redeemable by anyone.
//
// See the comment for NewBlockTemplate for more information about why the nil
// address handling is useful.
func createCoinbaseTx(nextBlockHeight uint64, payoutAddress chainutil.Address) (*chainutil.Tx, error) {
	tx := wire.NewMsgTx()
	tx.AddTxIn(&wire.TxIn{
		// Coinbase transactions have no inputs, so previous outpoint is
		// zero hash and max index.
		PreviousOutPoint: *wire.NewOutPoint(&wire.Hash{},
			wire.MaxPrevOutIndex),
		Sequence: wire.MaxTxInSequenceNum,
	})
	tx.SetPayload(standardCoinbasePayload(nextBlockHeight, 0))

	// Create a script for paying to the miner if one was specified.
	// Otherwise create a script that allows the coinbase to be
	// redeemable by anyone.
	pkScriptMiner := anyoneRedeemableScript
	var err error
	if payoutAddress != nil {
		pkScriptMiner, err = txscript.PayToAddrScript(payoutAddress)
		if err != nil {
			logging.CPrint(logging.ERROR, "createCoinbaseTx  pkScriptMiner",
				logging.LogFormat{
					"error":         err,
					"height":        nextBlockHeight,
					"payoutAddress": payoutAddress,
				})
			return nil, err
		}
	}
	miner, _, _, err := CalcBlockSubsidy(nextBlockHeight,
		&config.ChainParams, chainutil.ZeroAmount(), bitLengthMissing)
	if err != nil {
		logging.CPrint(logging.ERROR, "createCoinbaseTx  CalcBlockSubsidy",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight,
			})
		return nil, err
	}
	tx.AddTxOut(&wire.TxOut{
		Value:    miner.IntValue(),
		PkScript: pkScriptMiner,
	})
	return chainutil.NewTx(tx), nil
}

// calcNodeReward
func calcNodeReward(totalReward chainutil.Amount, totalWeight, nodeWeight *safetype.Uint128) (chainutil.Amount, error) {
	u, err := totalReward.Value().Mul(nodeWeight)
	if err != nil {
		return chainutil.ZeroAmount(), err
	}
	u, err = u.Div(totalWeight)
	if err != nil {
		return chainutil.ZeroAmount(), err
	}
	return chainutil.NewAmount(u)
}

// logSkippedDeps logs any dependencies which are also skipped as a result of
// skipping a transaction while generating a block template at the trace level.
func logSkippedDeps(tx *chainutil.Tx, deps *list.List) {
	if deps == nil {
		return
	}

	for e := deps.Front(); e != nil; e = e.Next() {
		item := e.Value.(*txPrioItem)
		logging.CPrint(logging.TRACE, "skipping tx since it depends on tx which is already skipped", logging.LogFormat{"txid": item.tx.Hash().String(), "depend": tx.Hash().String()})

	}
}

// BlockTemplateGenerator provides a type that can be used to generate block templates
// based on a given mining policy and source of transactions to choose from.
// It also houses additional state required in order to ensure the templates
// are built on top of the current best chain and adhere to the consensus rules.
type BlockTemplateGenerator struct {
	chain *Blockchain
}

// NewBlockTemplate returns a new block template that is ready to be solved
// using the transactions from the passed transaction memory pool and a coinbase
// that either pays to the passed address if it is not nil, or a coinbase that
// is redeemable by anyone if the passed wallet is nil.  The nil wallet
// functionality is useful since there are cases such as the getblocktemplate
// RPC where external mining software is responsible for creating their own
// coinbase which will replace the one generated for the block template.  Thus
// the need to have configured address can be avoided.
//
// The transactions selected and included are prioritized according to several
// factors.  First, each transaction has a priority calculated based on its
// value, age of inputs, and size.  Transactions which consist of larger
// amounts, older inputs, and small sizes have the highest priority.  Second, a
// fee per kilobyte is calculated for each transaction.  Transactions with a
// higher fee per kilobyte are preferred.  Finally, the block generation related
// configuration options are all taken into account.
//
// Transactions which only spend outputs from other transactions already in the
// block chain are immediately added to a priority queue which either
// prioritizes based on the priority (then fee per kilobyte) or the fee per
// kilobyte (then priority) depending on whether or not the BlockPrioritySize
// configuration option allots space for high-priority transactions.
// Transactions which spend outputs from other transactions in the memory pool
// are added to a dependency map so they can be added to the priority queue once
// the transactions they depend on have been included.
//
// Once the high-priority area (if configured) has been filled with transactions,
// or the priority falls below what is considered high-priority, the priority
// queue is updated to prioritize by fees per kilobyte (then priority).
//
// When the fees per kilobyte drop below the TxMinFreeFee configuration option,
// the transaction will be skipped unless there is a BlockMinSize set, in which
// case the block will be filled with the low-fee/free transactions until the
// block size reaches that minimum size.
//
// Any transactions which would cause the block to exceed the BlockMaxSize
// configuration option, exceed the maximum allowed signature operations per
// block, or otherwise cause the Block to be invalid are skipped.
//
// Given the above, a block generated by this function is of the following form:
//
//   -----------------------------------  --  --
//  |      Coinbase Transaction         |   |   |
//  |-----------------------------------|   |   |
//  |                                   |   |   | ----- cfg.BlockPrioritySize
//  |   High-priority Transactions      |   |   |
//  |                                   |   |   |
//  |-----------------------------------|   | --
//  |                                   |   |
//  |                                   |   |
//  |                                   |   |--- cfg.BlockMaxSize
//  |  Transactions prioritized by fee  |   |
//  |  until <= cfg.TxMinFreeFee        |   |
//  |                                   |   |
//  |                                   |   |
//  |                                   |   |
//  |-----------------------------------|   |
//  |  Low-fee/Non high-priority (free) |   |
//  |  transactions (while block size   |   |
//  |  <= cfg.BlockMinSize)             |   |
//   -----------------------------------  --

func (chain *Blockchain) NewBlockTemplate(payoutAddress chainutil.Address, templateCh chan interface{}) error {
	chain.l.Lock()
	defer chain.l.Unlock()
	// Get snapshot of chain/txPool
	bestNode := chain.blockTree.bestBlockNode()
	nextBlockHeight := bestNode.Height + 1
	txs := chain.txPool.TxDescs()
	punishments := chain.proposalPool.PunishmentProposals()
	var rewardAddresses []database.Rank
	// new epoch merge staking pool
	if nextBlockHeight >= consensus.StakingPoolAwardActivation && nextBlockHeight%consensus.StakingPoolMergeEpoch >= consensus.StakingPoolAwardStart {
		records, err := chain.db.FetchStakingAwardedRecordByTime(uint64(bestNode.Timestamp.Unix()))
		if err != nil {
			logging.CPrint(logging.ERROR, "NewBlockTemplate  FetchStakingAwardedRecordByTime",
				logging.LogFormat{
					"error":  err,
					"height": nextBlockHeight})
			return err
		}
		if len(records) == 0 {
			rewardAddresses, err = chain.db.FetchUnexpiredStakingRank(nextBlockHeight, true)
			if err != nil {
				logging.CPrint(logging.ERROR, "NewBlockTemplate  FetchUnexpiredStakingRank",
					logging.LogFormat{
						"error":  err,
						"height": nextBlockHeight})
				return err
			}
		}
	}
	go newBlockTemplate(chain, payoutAddress, templateCh, bestNode, txs, punishments, rewardAddresses)
	return nil
}

func GetFeeAfterBurnGas(fees chainutil.Amount) (chainutil.Amount, error) {
	//value, err := fees.Value().DivInt(2)
	//if err != nil {
	//	return chainutil.ZeroAmount(), err
	//}
	return chainutil.NewAmount(fees.Value())
}

func newBlockTemplate(chain *Blockchain, payoutAddress chainutil.Address, templateCh chan interface{},
	bestNode *BlockNode, mempoolTxns []*TxDesc, proposals []*PunishmentProposal, rewardAddresses []database.Rank) {
	nextBlockHeight := bestNode.Height + 1
	challenge, err := calcNextChallenge(bestNode)
	if err != nil {
		logging.CPrint(logging.ERROR, "newBlockTemplate  calcNextChallenge",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		templateCh <- &PoCTemplate{
			Err: err,
		}
		return
	}
	coinbaseTx, err := createCoinbaseTx(nextBlockHeight, payoutAddress)
	if err != nil {
		logging.CPrint(logging.ERROR, "newBlockTemplate  createCoinbaseTx",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		templateCh <- &PoCTemplate{
			Err: err,
		}
		return
	}
	governConfig, err := chain.chainGovern.FetchEnabledGovernConfig(GovernSenateClass, nextBlockHeight)
	if err != nil {
		logging.CPrint(logging.ERROR, "newBlockTemplate  FetchEnabledGovernanceConfig",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		templateCh <- &PoCTemplate{
			Err: ErrBadTxOutValue,
		}
		return
	}
	nodesConfig, ok := governConfig.(*GovernSenateConfig)
	if !ok {
		logging.CPrint(logging.ERROR, "newBlockTemplate  get GovernanceSenateNodesConfig",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		templateCh <- &PoCTemplate{
			Err: ErrBadTxOutValue,
		}
		return
	}

	getCoinbaseTx := func(pubkey *pocec.PublicKey, totalFee chainutil.Amount, bitLength int) (*chainutil.Tx, error) {
		pkScriptHash, err := pkToScriptHash(pubkey.SerializeCompressed(), &config.ChainParams)
		if err != nil {
			logging.CPrint(logging.ERROR, "newBlockTemplate  getCoinbaseTx  pkToScriptHash",
				logging.LogFormat{
					"error":  err,
					"pubkey": pubkey.SerializeCompressed(),
					"height": nextBlockHeight})
			return nil, err
		}

		BindingTxListReply, err := chain.db.FetchScriptHashRelatedBindingTx(pkScriptHash, &config.ChainParams)
		if err != nil {
			logging.CPrint(logging.ERROR, "newBlockTemplate  getCoinbaseTx  FetchScriptHashRelatedBindingTx",
				logging.LogFormat{
					"error":  err,
					"pubkey": pubkey.SerializeCompressed(),
					"height": nextBlockHeight})
			return nil, err
		}

		err = reCreateCoinbaseTx(coinbaseTx.MsgTx(), BindingTxListReply, nextBlockHeight, bitLength, rewardAddresses, nodesConfig.senates, totalFee)
		if err != nil {
			logging.CPrint(logging.ERROR, "newBlockTemplate  getCoinbaseTx  reCreateCoinbaseTx",
				logging.LogFormat{
					"error":  err,
					"pubkey": pubkey.SerializeCompressed(),
					"height": nextBlockHeight})
			return nil, err
		}
		return coinbaseTx, nil
	}

	getTarget := func(timestamp time.Time) *big.Int {
		target, _ := calcNextTarget(bestNode, timestamp)
		return target
	}
	templateCh <- &PoCTemplate{
		Height:        nextBlockHeight,
		Timestamp:     bestNode.Timestamp.Add(1 * poc.PoCSlot * time.Second),
		Previous:      *bestNode.Hash,
		Challenge:     *challenge,
		GetTarget:     getTarget,
		RewardAddress: rewardAddresses,
		GetCoinbase:   getCoinbaseTx,
	}

	//  after create coinbase transaction

	numCoinbaseSigOps := int64(CountSigOps(coinbaseTx))

	// Create slices to hold the fees and number of signature operations
	// for each of the selected transactions and add an entry for the
	// coinbase.  This allows the code below to simply append details about
	// a transaction as it is selected for inclusion in the final block.
	// However, since the total fees aren't known yet, use a dummy value for
	// the coinbase fee which will be updated later.
	txSigOpCounts := make([]int64, 0, len(mempoolTxns))
	txSigOpCounts = append(txSigOpCounts, numCoinbaseSigOps)

	// Get the current memory pool transactions and create a priority queue
	// to hold the transactions which are ready for inclusion into a block
	// along with some priority related and fee metadata.  Reserve the same
	// number of items that are in the memory pool for the priority queue.
	// Also, choose the initial sort order for the priority queue based on
	// whether or not there is an area allocated for high-priority
	// transactions.
	depMap := make(map[wire.Hash]struct{})
	for _, txDesc := range mempoolTxns {
		txHash := txDesc.Tx.Hash()
		if _, exist := depMap[*txHash]; !exist {
			depMap[*txHash] = struct{}{}
		}
	}
	sortedByFee := true //config.BlockPrioritySize == 0
	priorityQueue := newTxPriorityQueue(len(mempoolTxns), sortedByFee)

	// Create a slice to hold the transactions to be included in the
	// generated block with reserved space.  Also create a transaction
	// store to house all of the input transactions so multiple lookups
	// can be avoided.
	blockTxns := make([]*wire.MsgTx, 0, len(mempoolTxns)+1)
	blockTxns = append(blockTxns, coinbaseTx.MsgTx())

	if len(rewardAddresses) > 0 || nextBlockHeight%consensus.StakingPoolMergeEpoch == 0 {
		// get unspent staking pool tx
		var startHeight = bestNode.Height
		if startHeight > consensus.CoinbaseMaturity {
			startHeight = startHeight - consensus.CoinbaseMaturity
			if startHeight > consensus.StakingPoolMergeEpoch {
				startHeight = startHeight - consensus.StakingPoolMergeEpoch
			} else {
				startHeight = 0
			}
		} else {
			startHeight = 0
		}

		blockShaList, err := chain.db.FetchHeightRange(startHeight, nextBlockHeight)
		if err != nil {
			logging.CPrint(logging.ERROR, "newBlockTemplate  FetchHeightRange",
				logging.LogFormat{
					"error":        err,
					"start height": startHeight,
					"height":       nextBlockHeight})
			templateCh <- &PoCTemplate{
				Err: err,
			}
			return
		}

		unspentTxShaList := make([]*wire.Hash, 0)
		for _, blockSha := range blockShaList {
			block, err := chain.db.FetchBlockBySha(&blockSha)
			if err != nil {
				logging.CPrint(logging.ERROR, "newBlockTemplate  FetchBlockBySha",
					logging.LogFormat{
						"error":        err,
						"start height": startHeight,
						"block sha":    blockSha,
						"height":       nextBlockHeight})
				templateCh <- &PoCTemplate{
					Err: err,
				}
				return
			}
			unspentTxShaList = append(unspentTxShaList, block.Transactions()[0].Hash())
		}
		unspentStakingPoolTxs := chain.db.FetchUnSpentTxByShaList(unspentTxShaList)
		if len(unspentStakingPoolTxs) > 0 {
			if len(rewardAddresses) > 0 {
				stakingPoolRewardTx, err := createStakingPoolRewardTx(nextBlockHeight, rewardAddresses, unspentStakingPoolTxs)
				if err != nil {
					logging.CPrint(logging.ERROR, "newBlockTemplate  createStakingPoolRewardTx",
						logging.LogFormat{
							"error":        err,
							"start height": startHeight,
							"height":       nextBlockHeight})
					templateCh <- &PoCTemplate{
						Err: err,
					}
					return
				}
				blockTxns = append(blockTxns, stakingPoolRewardTx.MsgTx())
				numStakingPoolRewardTxSigOps := int64(CountSigOps(stakingPoolRewardTx))
				txSigOpCounts = append(txSigOpCounts, numStakingPoolRewardTxSigOps)
			} else {
				stakingPoolMergeTx, err := createStakingPoolMergeTx(nextBlockHeight, unspentStakingPoolTxs)
				if err != nil {
					logging.CPrint(logging.ERROR, "newBlockTemplate  createStakingPoolMergeTx",
						logging.LogFormat{
							"error":        err,
							"start height": startHeight,
							"height":       nextBlockHeight})
					templateCh <- &PoCTemplate{
						Err: err,
					}
					return
				}
				if stakingPoolMergeTx != nil {
					blockTxns = append(blockTxns, stakingPoolMergeTx.MsgTx())
					numStakingPoolRewardTxSigOps := int64(CountSigOps(stakingPoolMergeTx))
					txSigOpCounts = append(txSigOpCounts, numStakingPoolRewardTxSigOps)
				}
			}
		}
	}
	//blockTxStore := make(TxStore)
	punishProposals := make([]*wire.FaultPubKey, 0, len(proposals))
	otherProposals := make([]*wire.NormalProposal, 0)

	var totalProposalsSize int
	totalProposalsSize += 4

	// dependers is used to track transactions which depend on another
	// transaction in the memory pool.  This, in conjunction with the
	// dependsOn map kept with each dependent transaction helps quickly
	// determine which dependent transactions are now eligible for inclusion
	// in the block once each transaction has been included.
	dependers := make(map[wire.Hash]*list.List)

	logging.CPrint(logging.DEBUG, "Considering transactions in mempool for inclusion to new block", logging.LogFormat{"tx count": len(mempoolTxns)})

	banList := make([]*pocec.PublicKey, 0)
	if len(proposals) != 0 {
		for _, proposal := range proposals {
			if totalProposalsSize+proposal.Size <= PriorityProposalSize {
				totalProposalsSize += proposal.Size
				punishProposals = append(punishProposals, proposal.FaultPubKey)
				banList = append(banList, proposal.PubKey)
			}
		}
	} else {
		totalProposalsSize += wire.HeaderSizePerPlaceHolder * 2
	}

	//mempoolLoop:
	for _, txDesc := range mempoolTxns {
		// A block can't have more than one coinbase or contain
		// non-finalized transactions.
		tx := txDesc.Tx

		// Setup dependencies for any transactions which reference
		// other transactions in the mempool so they can be properly
		// ordered below.
		prioItem := &txPrioItem{tx: txDesc.Tx}
		for _, txIn := range tx.MsgTx().TxIn {
			originHash := &txIn.PreviousOutPoint.Hash
			// because mempoolTxns is snapshot of mempool, its tx can not orphan
			if _, exist := depMap[*originHash]; exist {
				// The transaction is referencing another
				// transaction in the memory pool, so setup an
				// ordering dependency.
				depList, exists := dependers[*originHash]
				if !exists {
					depList = list.New()
					dependers[*originHash] = depList
				}
				depList.PushBack(prioItem)
				if prioItem.dependsOn == nil {
					prioItem.dependsOn = make(
						map[wire.Hash]struct{})
				}
				prioItem.dependsOn[*originHash] = struct{}{}
			}
		}

		// Calculate the final transaction priority using the input
		// value age sum as well as the adjusted transaction size.  The
		// formula is: sum(inputValue * inputAge) / adjustedTxSize
		prioItem.priority = float64(txDesc.totalInputValue.IntValue())*(float64(nextBlockHeight)-float64(txDesc.Height)) + txDesc.startingPriority

		// Calculate the fee in Sukhavati/KB.
		// NOTE: This is a more precise value than the one calculated
		// during calcMinRelayFee which rounds up to the nearest full
		// kilobyte boundary.  This is beneficial since it provides an
		// incentive to create smaller transactions.
		txSize := tx.MsgTx().PlainSize()
		prioItem.feePerKB = float64(txDesc.Fee.IntValue()) / (float64(txSize) / 1000)
		prioItem.fee = txDesc.Fee.IntValue()

		// Add the transaction to the priority queue to mark it ready
		// for inclusion in the block unless it has dependencies.
		if prioItem.dependsOn == nil {
			heap.Push(priorityQueue, prioItem)
		}
	}

	logging.CPrint(logging.TRACE, "Check the length of priority queue and dependent",
		logging.LogFormat{"priority": priorityQueue.Len(), "dependent": len(dependers)})

	// The starting block size is the size of the Block header plus the max
	// possible transaction count size, plus the size of the coinbase
	// transaction.
	//modify: 360 is coinbase`s binding txIn
	blockSize := uint32(blockHeaderOverhead+int64(len(punishProposals)*33)+int64(coinbaseTx.PlainSize())+int64(totalProposalsSize)) + bindingPayload
	blockSigOps := numCoinbaseSigOps
	totalFee := chainutil.ZeroAmount()

	// Choose which transactions make it into the block.
	for priorityQueue.Len() > 0 {
		// Grab the highest priority (or highest fee per kilobyte
		// depending on the sort order) transaction.
		prioItem := heap.Pop(priorityQueue).(*txPrioItem)
		tx := prioItem.tx

		// Grab the list of transactions which depend on this one (if
		// any) and remove the entry for this transaction as it will
		// either be included or skipped, but in either case the deps
		// are no longer needed.
		deps := dependers[*tx.Hash()]
		delete(dependers, *tx.Hash())
		// Enforce maximum block size.  Also check for overflow.
		txSize := uint32(tx.PlainSize())
		blockPlusTxSize := blockSize + txSize
		// Enforce maximum block size.  Also check for overflow.

		if blockPlusTxSize < blockSize ||
			blockPlusTxSize >= config.BlockMaxSize {
			logging.CPrint(logging.TRACE, "Skipping tx because it would exceed the max block weight",
				logging.LogFormat{"txid": tx.Hash().String()})
			logSkippedDeps(tx, deps)
			continue
		}

		// Enforce maximum signature operations per block.  Also check
		// for overflow.
		numSigOps := int64(CountSigOps(tx))
		if blockSigOps+numSigOps < blockSigOps || blockSigOps+numSigOps > MaxSigOpsPerBlock {
			logging.CPrint(logging.TRACE, "Skipping tx because it would exceed the maximum sigops per block",
				logging.LogFormat{"txid": tx.Hash().String()})
			logSkippedDeps(tx, deps)
			continue
		}

		// Skip free transactions once the block is larger than the
		// minimum block size.
		if sortedByFee &&
			prioItem.feePerKB < float64(consensus.MinRelayTxFee) &&
			blockPlusTxSize >= config.BlockMinSize {

			logging.CPrint(logging.TRACE, "Skipping tx with feePerKB < TxMinFreeFee and block weight >= minBlockSize",
				logging.LogFormat{"txid": tx.Hash().String(), "feePerKB": prioItem.feePerKB, "TxMinFreeFee": consensus.MinRelayTxFee, "block weight": blockPlusTxSize, "minBlockSize": config.BlockMinSize})
			logSkippedDeps(tx, deps)
			continue
		}

		// Prioritize by fee per kilobyte once the block is larger than
		// the priority size or there are no more high-priority
		// transactions.
		if !sortedByFee && (blockPlusTxSize >= config.BlockPrioritySize ||
			prioItem.priority <= minHighPriority) {

			logging.CPrint(logging.TRACE, "Switching to sort by fees per kilobyte since blockSize >= BlockPrioritySize || priority <= minHighPriority",
				logging.LogFormat{
					"block size":        blockPlusTxSize,
					"BlockPrioritySize": config.BlockPrioritySize,
					"priority":          fmt.Sprintf("%.2f", prioItem.priority),
					"minHighPriority":   fmt.Sprintf("%.2f", minHighPriority)})

			sortedByFee = true
			priorityQueue.SetLessFunc(txPQByFee)

			// Put the transaction back into the priority queue and
			// skip it so it is re-priortized by fees if it won't
			// fit into the high-priority section or the priority
			// is too low.  Otherwise this transaction will be the
			// final one in the high-priority section, so just fall
			// though to the code below so it is added now.
			if blockPlusTxSize > config.BlockPrioritySize ||
				prioItem.priority < minHighPriority {

				heap.Push(priorityQueue, prioItem)
				continue
			}
		}

		temp, err := totalFee.AddInt(prioItem.fee)
		if err != nil {
			logging.CPrint(logging.ERROR, "calc total fee error",
				logging.LogFormat{
					"err":  err,
					"txid": tx.Hash().String(),
				})
			continue
		}

		// Add the transaction to the block, increment counters, and
		// save the fees and signature operation counts to the block
		// template.
		blockTxns = append(blockTxns, tx.MsgTx())
		blockSize += txSize
		blockSigOps += numSigOps
		totalFee = temp
		txSigOpCounts = append(txSigOpCounts, numSigOps)

		logging.CPrint(logging.TRACE, "Adding tx",
			logging.LogFormat{"txid": tx.Hash().String(),
				"priority": fmt.Sprintf("%.2f", prioItem.priority),
				"feePerKB": fmt.Sprintf("%.2f", prioItem.feePerKB)})

		// Add transactions which depend on this one (and also do not
		// have any other unsatisfied dependencies) to the priority
		// queue.
		if deps != nil {
			for e := deps.Front(); e != nil; e = e.Next() {
				// Add the transaction to the priority queue if
				// there are no more dependencies after this
				// one.
				item := e.Value.(*txPrioItem)
				delete(item.dependsOn, *tx.Hash())
				if len(item.dependsOn) == 0 {
					heap.Push(priorityQueue, item)
				}
			}
		}
	}

	// Next, obtain the merkle root of a tree which consists of the
	// wtxid of all transactions in the block. The coinbase
	// transaction will have a special wtxid of all zeroes.
	witnessMerkleTree := wire.BuildMerkleTreeStoreTransactions(blockTxns, true)
	witnessMerkleRoot := *witnessMerkleTree[len(witnessMerkleTree)-1]

	// Create a new block ready to be solved.
	// we will get chainID, Version, Height, Previous, TransactionRoot, CommitRoot,
	// ProposalRoot, BanList, tx and proposal
	merkles := wire.BuildMerkleTreeStoreTransactions(blockTxns, false)
	var msgBlock = wire.NewEmptyMsgBlock()
	msgBlock.Header = *wire.NewEmptyBlockHeader()
	msgBlock.Header.ChainID = bestNode.ChainID
	msgBlock.Header.Version = generatedBlockVersion
	msgBlock.Header.Height = nextBlockHeight
	msgBlock.Header.Previous = *bestNode.Hash
	msgBlock.Header.TransactionRoot = *merkles[len(merkles)-1]
	msgBlock.Header.WitnessRoot = witnessMerkleRoot

	merklesCache := make([]*wire.Hash, 0)
	merklesCache = append(merklesCache, merkles[0])
	if len(merkles) > 1 {
		base := (len(merkles) + 1) / 2
		for i := 1; i < len(merkles) && base > 0; {
			merklesCache = append(merklesCache, merkles[i])
			i += base
			base = base / 2
		}
	}

	witnessMerklesCache := make([]*wire.Hash, 0)
	witnessMerklesCache = append(witnessMerklesCache, witnessMerkleTree[0])
	if len(witnessMerkleTree) > 1 {
		base := (len(witnessMerkleTree) + 1) / 2
		for i := 1; i < len(witnessMerkleTree) && base > 0; {
			witnessMerklesCache = append(witnessMerklesCache, witnessMerkleTree[i])
			i += base
			base = base / 2
		}
	}

	proposalArea, err := wire.NewProposalArea(punishProposals, otherProposals)
	if err != nil {
		logging.CPrint(logging.ERROR, "newBlockTemplate  NewProposalArea",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		templateCh <- &BlockTemplate{
			Err: err,
		}
		return
	}
	proposalMerkles := wire.BuildMerkleTreeStoreForProposal(proposalArea)
	proposalRoot := proposalMerkles[len(proposalMerkles)-1]
	msgBlock.Header.ProposalRoot = *proposalRoot
	msgBlock.Proposals = *proposalArea

	msgBlock.Header.BanList = banList

	for _, tx := range blockTxns {
		msgBlock.AddTransaction(tx)
	}

	// Finally, perform a full check on the created block against the chain
	// consensus rules to ensure it properly connects to the current best
	// chain with no issues.
	block := chainutil.NewBlock(msgBlock)
	block.SetHeight(nextBlockHeight)

	//if !msgBlock.Header.Previous.IsEqual(chain.GetBestChainHash()) {
	//	return nil, errors.New("block template stale")
	//}

	//timeSource := NewMedianTime()
	if _, err := chain.execProcessBlock(block, BFNoPoCCheck); err != nil {
		logging.CPrint(logging.ERROR, "createStakingPoolRewardTx  execProcessBlock",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		templateCh <- &BlockTemplate{
			Err: err,
		}
		return
	}
	totalFee, err = GetFeeAfterBurnGas(totalFee)
	if err != nil {
		logging.CPrint(logging.ERROR, "newBlockTemplate  GetFeeAfterBurnGas",
			logging.LogFormat{
				"error":  err,
				"height": nextBlockHeight})
		templateCh <- &BlockTemplate{
			Err: err,
		}
		return
	}
	logging.CPrint(logging.DEBUG, "Created new block template",
		logging.LogFormat{
			"tx count":                  len(msgBlock.Transactions),
			"total fee":                 totalFee,
			"signature operations cost": blockSigOps,
			"block size":                blockSize,
			"target difficulty":         fmt.Sprintf("%064x", msgBlock.Header.Target)})
	templateCh <- &BlockTemplate{
		Block:              msgBlock,
		TotalFee:           totalFee,
		SigOpCounts:        txSigOpCounts,
		Height:             nextBlockHeight,
		ValidPayAddress:    payoutAddress != nil,
		MerkleCache:        merklesCache,
		WitnessMerkleCache: witnessMerklesCache,
	}
}
