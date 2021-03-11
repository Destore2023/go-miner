package config_test

import (
	"encoding/hex"
	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/txscript"
	"testing"
)

func TestGenesisDoc(t *testing.T) {
	address, err := chainutil.DecodeAddress("sk1qqrz45pn0x7nmqsl386yv8z77gpstkchzdzmfrppprazfk3xudrq3samq95a", &config.ChainParams)
	if err == nil {
		script, err := txscript.PayToAddrScript(address)
		if err != nil {
			return
		}
		toString := hex.EncodeToString(script)
		print(toString)
	}
}
