package blockchain

import (
	"fmt"
	"time"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

// blockExists determines whether a block with the given hash exists either in
// the main chain or any side chains.
//
// This function is safe for concurrent access.
func (chain *Blockchain) blockExists(hash *wire.Hash) bool {
	// Check memory chain first (could be main chain or side chain blocks).
	if chain.blockTree.nodeExists(hash) {
		return true
	}
	// Check in database (rest of main chain not in memory).
	exists, err := chain.db.ExistsBlockSha(hash)
	if err != nil {
		logging.CPrint(logging.ERROR, "fail to check block existence from db",
			logging.LogFormat{"hash": hash, "err": err})
	}
	return exists
}

func (chain *Blockchain) processOrphans(hash *wire.Hash, flags BehaviorFlags) error {
	for _, orphan := range chain.blockTree.orphanBlockPool.getOrphansByPrevious(hash) {
		logging.CPrint(logging.INFO, "process orphan",
			logging.LogFormat{
				"parent_hash":  hash,
				"child_hash":   orphan.block.Hash(),
				"child_height": orphan.block.Height(),
			})
		if err := chain.maybeAcceptBlock(orphan.block, BFNone); err != nil {
			chain.errCache.Add(orphan.block.Hash().String(), err)
			return err
		}

		chain.blockTree.orphanBlockPool.removeOrphanBlock(orphan)

		if err := chain.processOrphans(orphan.block.Hash(), BFNone); err != nil {
			return err
		}
	}

	return nil
}

func (chain *Blockchain) processBlock(block *chainutil.Block, flags BehaviorFlags) (isOrphan bool, err error) {
	var startProcessing = time.Now()

	if flags.isFlagSet(BFNoPoCCheck) {
		// Perform preliminary sanity checks on the block and its transactions.
		err := checkBlockSanity(block, chain.info.chainID, config.ChainParams.PocLimit, flags)
		if err != nil {
			return false, err
		}

		// The block has passed all context independent checks and appears sane
		// enough to potentially accept it into the block chain.
		if err := chain.maybeAcceptBlock(block, flags); err != nil {
			logging.CPrint(logging.ERROR, "fail on maybeAcceptBlock with BFNoPoCCheck", logging.LogFormat{
				"err":      err,
				"previous": block.MsgBlock().Header.Previous,
				"height":   block.Height(),
				"elapsed":  time.Since(startProcessing),
				"flags":    fmt.Sprintf("%b", flags),
			})
			return false, err
		}
		return false, nil
	}

	blockHash := block.Hash()
	logging.CPrint(logging.TRACE, "processing block", logging.LogFormat{
		"hash":     blockHash,
		"height":   block.Height(),
		"tx_count": len(block.Transactions()),
		"flags":    fmt.Sprintf("%b", flags),
	})

	// The block must not already exist in the main chain or side chains.
	if chain.blockExists(blockHash) {
		return false, nil
	}

	// The block must not already exist as an orphan.
	if chain.blockTree.orphanExists(blockHash) {
		return true, nil
	}

	// Return fail if block error already been cached.
	if v, ok := chain.errCache.Get(blockHash.String()); ok {
		return false, v.(error)
	}

	// Perform preliminary sanity checks on the block and its transactions.
	err = checkBlockSanity(block, chain.info.chainID, config.ChainParams.PocLimit, flags)
	if err != nil {
		chain.errCache.Add(blockHash.String(), err)
		return false, err
	}

	blockHeader := &block.MsgBlock().Header

	// Handle orphan blocks.
	prevHash := &blockHeader.Previous
	if !prevHash.IsEqual(zeroHash) {
		prevHashExists := chain.blockExists(prevHash)

		if !prevHashExists {
			logging.CPrint(logging.INFO, "Adding orphan block with Parent", logging.LogFormat{
				"orphan":  blockHash,
				"height":  block.Height(),
				"parent":  prevHash,
				"elapsed": time.Since(startProcessing),
				"flags":   fmt.Sprintf("%b", flags),
			})
			chain.blockTree.orphanBlockPool.addOrphanBlock(block)
			return true, nil
		}
	}

	// The block has passed all context independent checks and appears sane
	// enough to potentially accept it into the block chain.
	if err := chain.maybeAcceptBlock(block, flags); err != nil {
		chain.errCache.Add(blockHash.String(), err)
		return false, err
	}

	// Accept any orphan blocks that depend on this block (they are
	// no longer orphans) and repeat for those accepted blocks until
	// there are no more.
	if err := chain.processOrphans(blockHash, flags); err != nil {
		return false, err
	}

	logging.CPrint(logging.DEBUG, "accepted block", logging.LogFormat{
		"hash":     blockHash,
		"height":   block.Height(),
		"tx_count": len(block.Transactions()),
		"elapsed":  time.Since(startProcessing),
		"flags":    fmt.Sprintf("%b", flags),
	})

	return false, nil
}
