package mock

import (
	"fmt"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

func (c *Chain) constructBlock(block *wire.MsgBlock, hgt uint64) (*wire.MsgBlock, error) {
	if block.Header.Height != hgt {
		return nil, fmt.Errorf("invalid height %d:%d", block.Header.Height, hgt)
	}
	block.Transactions = make([]*wire.MsgTx, 0)
	// construct coinbase tx
	coinbase, coinbaseHash, err := c.createCoinbaseTx(block.Header.Height, block.Header.PubKey)
	if err != nil {
		return nil, err
	}
	block.AddTransaction(coinbase.MsgTx())

	blockStat := &BlockTxStat{
		height: block.Header.Height,
	}

	if block.Header.Height >= threshold {
		// create non-coinbase txs
		txs, stat, totalFee, err := c.constructTxs1(block.Header.Height)
		if err != nil {
			return nil, err
		}
		if totalFee < 0 {
			return nil, fmt.Errorf("negative total fee at block %d", block.Header.Height)
		}
		minerTxOut := coinbase.MsgTx().TxOut[len(coinbase.MsgTx().TxOut)-1]
		minerTxOut.Value += totalFee

		for _, tx := range txs {
			block.AddTransaction(tx)
		}
		blockStat.stat = stat
	}

	// re calculate utxo
	c.reCalcCoinbaseUtxo(block, coinbaseHash)

	// fill block header
	err = c.fillBlockHeader(block)
	if err != nil {
		return nil, err
	}
	c.stats = append(c.stats, blockStat)
	c.constructHeight = hgt
	return block, nil
}

func (c *Chain) fillBlockHeader(block *wire.MsgBlock) error {
	// fill previous
	block.Header.Previous = c.blocks[block.Header.Height-1].BlockHash()

	// fill transaction root
	merkles := wire.BuildMerkleTreeStoreTransactions(block.Transactions, false)
	witnessMerkles := wire.BuildMerkleTreeStoreTransactions(block.Transactions, true)
	block.Header.TransactionRoot = *merkles[len(merkles)-1]
	block.Header.WitnessRoot = *witnessMerkles[len(witnessMerkles)-1]

	// fill sig2
	key := pocKeys[pkStr(block.Header.PubKey)]
	pocHash, err := block.Header.PoCHash()
	if err != nil {
		return err
	}
	data := wire.HashH(pocHash[:])
	sig, err := key.Sign(data[:])
	if err != nil {
		return err
	}
	block.Header.Signature = sig

	return nil
}

func copyBlock(block *wire.MsgBlock) (*wire.MsgBlock, error) {
	buf, err := chainutil.NewBlock(block).Bytes(wire.Packet)
	if err != nil {
		return nil, err
	}
	newBlock, err := chainutil.NewBlockFromBytes(buf, wire.Packet)
	if err != nil {
		return nil, err
	}
	return newBlock.MsgBlock(), nil
}
