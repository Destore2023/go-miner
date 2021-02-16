package wire

import (
	"bytes"
	"reflect"
	"testing"
)

// TestBlock tests the MsgBlock API.
func TestBlock(t *testing.T) {
	var testRound = 500

	for i := 1; i < testRound; i += 20 {
		block := mockBlock(2000 / i)
		var wBuf bytes.Buffer
		n, err := block.Encode(&wBuf, DB)
		if err != nil {
			t.Fatal(i, err, n)
		}

		newBlock := new(MsgBlock)
		m, err := newBlock.Decode(&wBuf, DB)
		if err != nil {
			t.Fatal(i, err, m, block.Proposals.PunishmentCount(), block.Proposals.OtherCount(), block.Proposals.PlainSize())
		}

		// compare block and newBlock
		if !reflect.DeepEqual(block, newBlock) {
			t.Error("block and newBlock is not equal")
			if !reflect.DeepEqual(block.Header, newBlock.Header) {
				t.Error("header not equal")
			}
			if !reflect.DeepEqual(block.Proposals, newBlock.Proposals) {
				t.Error("pa not equal")
			}
			if len(block.Transactions) != len(newBlock.Transactions) {
				t.Error("txs not equal", len(block.Transactions), len(newBlock.Transactions))
				for j := 0; j < len(block.Transactions); j++ {
					if !reflect.DeepEqual(block.Transactions[j], newBlock.Transactions[j]) {
						t.Error("tx not equal", j)
					}
				}
			}
			t.FailNow()
		}

		bs, err := block.Bytes(Plain)
		if err != nil {
			t.Fatal(i, err)
		}
		// compare size
		if size := block.PlainSize(); (block.Proposals.PunishmentCount() == 0 && len(bs)+PlaceHolderSize != size) || (block.Proposals.PunishmentCount() != 0 && len(bs) != size) {
			t.Fatal(i, "block size incorrect", n, m, len(bs), size)
		}
	}
}
