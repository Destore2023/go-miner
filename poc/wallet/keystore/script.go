package keystore

import (
	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/pocec"
)

func NewPoCAddress(pubKey *pocec.PublicKey, net *config.Params) ([]byte, chainutil.Address, error) {

	//verify nil pointer,avoid panic error
	if pubKey == nil {
		return nil, nil, ErrNilPointer
	}
	scriptHash, pocAddress, err := newPoCAddress(pubKey, net)
	if err != nil {
		logging.CPrint(logging.ERROR, "newPoCAddress failed",
			logging.LogFormat{
				"err": err,
			})
		return nil, nil, err
	}

	return scriptHash, pocAddress, nil
}

func newPoCAddress(pubKey *pocec.PublicKey, net *config.Params) ([]byte, chainutil.Address, error) {

	scriptHash := chainutil.Hash160(pubKey.SerializeCompressed())

	addressPubKeyHash, err := chainutil.NewAddressPubKeyHash(scriptHash, net)
	if err != nil {
		return nil, nil, err
	}

	return scriptHash, addressPubKeyHash, nil
}
