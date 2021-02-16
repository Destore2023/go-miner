package database_test

import (
	"fmt"
	"testing"

	"github.com/Sukhavati-Labs/go-miner/database"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/config"
)

func TestName(t *testing.T) {

	address, err := chainutil.DecodeAddress("sk1qqggu42p34335mwrutv88t7fqh6sp5eqlawglmx457dhn0w7ks2nzsm0rq7q", &config.ChainParams)
	if err != nil {
		fmt.Println(err)
	}
	equity := database.SenateEquity{
		Equity: 10000,
	}
	copy(equity.ScriptHash[:], address.ScriptAddress())
	config := &database.GovernanceSenateNodesConfig{
		SenateEquities: database.SenateEquities{equity},
	}

	data, err := config.Bytes()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("data:" + string(data))
	governanceConfig, err := database.DecodeGovernanceConfig(data)
	if err != nil {
		fmt.Println(err)
	}
	governanceConfig.GetHeight()

}
