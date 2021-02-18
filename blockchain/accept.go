package blockchain

import (
	"fmt"
	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/logging"
)

// maybeAcceptBlock potentially accepts a block into the block chain and, if
// accepted, returns whether or not it is on the main chain.  It performs
// several validation checks which depend on its position within the block chain
// before adding it.  The block is expected to have already gone through
// ProcessBlock before calling this function with it.
//
// The flags are also passed to checkBlockContext and connectBestChain.  See
// their documentation for how the flags modify their behavior.
//
// This function MUST be called with the chain state lock held (for writes).
func (chain *Blockchain) maybeAcceptBlock(block *chainutil.Block, flags BehaviorFlags) error {
	// Get a block node for the block previous to this one.  Will be nil
	// if this is the genesis block.
	prevNode, err := chain.getPrevNodeFromBlock(block)
	if err != nil {
		logging.CPrint(logging.ERROR, "fail on getPrevNodeFromBlock",
			logging.LogFormat{
				"err":     err,
				"preHash": block.MsgBlock().Header.Previous,
				"hash":    block.Hash(),
				"flags":   fmt.Sprintf("%b", flags),
			})
		return err
	}
	if prevNode == nil {
		logging.CPrint(logging.ERROR, "prev node not found",
			logging.LogFormat{
				"preHash": block.MsgBlock().Header.Previous,
				"hash":    block.Hash(),
				"flags":   fmt.Sprintf("%b", flags),
			})
		return fmt.Errorf("prev node not found")
	}

	// The height of this block is one more than the referenced previous
	// block.
	block.SetHeight(prevNode.Height + 1)

	// The block must pass all of the validation rules which depend on the
	// position of the block within the block chain.
	err = chain.checkBlockContext(block, prevNode, flags)
	if err != nil {
		logging.CPrint(logging.ERROR, "fail on checkBlockContext",
			logging.LogFormat{
				"err": err, "preHash": block.MsgBlock().Header.Previous,
				"hash":  block.Hash(),
				"flags": fmt.Sprintf("%b", flags),
			})
		return err
	}

	// Create a new block node for the block and add it to the in-memory
	// block chain (could be either a side chain or the main chain).
	blockHeader := &block.MsgBlock().Header

	if flags.isFlagSet(BFNoPoCCheck) {
		return chain.checkConnectBlock(NewBlockNode(blockHeader, nil, BFNoPoCCheck), block)
	}

	newNode := NewBlockNode(blockHeader, block.Hash(), BFNone)
	newNode.Parent = prevNode

	// Connect the passed block to the chain while respecting proper chain
	// selection according to the chain with the most proof of work.  This
	// also handles validation of the transaction scripts.
	err = chain.connectBestChain(newNode, block, flags)
	if err != nil {
		return err
	}

	return nil
}
