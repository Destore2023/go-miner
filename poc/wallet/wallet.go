package wallet

import (
	"fmt"
	"github.com/Sukhavati-Labs/go-miner/config"
	"os"

	"github.com/Sukhavati-Labs/go-miner/poc/wallet/db"

	"github.com/Sukhavati-Labs/go-miner/poc/wallet/keystore"
)

type PoCWallet struct {
	*keystore.KeystoreManagerForPoC
	store db.DB
}

func NewPoCWallet(cfg *PoCWalletConfig, password []byte) (*PoCWallet, error) {
	dbPath := cfg.DBPath()

	var store db.DB
	if fi, err := os.Stat(dbPath); err == nil {
		if !fi.IsDir() {
			return nil, fmt.Errorf("open %s: not a directory", dbPath)
		}
		if store, err = db.OpenDB(cfg.dbType, dbPath); err != nil {
			return nil, err
		}
	} else if os.IsNotExist(err) {
		if store, err = db.CreateDB(cfg.dbType, dbPath); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	manager, err := keystore.NewKeystoreManagerForPoC(store, password, &config.ChainParams)
	if err != nil {
		store.Close()
		return nil, err
	}
	return &PoCWallet{
		KeystoreManagerForPoC: manager,
		store:                 store,
	}, nil
}

func (walllet *PoCWallet) ImportOtherWallet(otherWallet *PoCWallet, privPass []byte) (map[string]string, error) {
	err := otherWallet.Unlock(privPass)
	if err != nil {
		return nil, err
	}
	return walllet.ImportManagedAddrManager(otherWallet.KeystoreManagerForPoC)
}

func (wallet *PoCWallet) Close() error {
	return wallet.store.Close()
}
