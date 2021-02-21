package keystore

import (
	"errors"
	"strconv"

	"github.com/Sukhavati-Labs/go-miner/poc/wallet/keystore/hdkeychain"
)

var (
	TestnetKeyScope = KeyScope{
		Purpose: 44,
		Coin:    1,
	}

	MainnetKeyScope = KeyScope{
		Purpose: 44,
		Coin:    2021,
	}

	PocDerivationPath = DerivationPath{
		Account: 0,
		Branch:  0,
	}

	WalletDerivationPath = DerivationPath{
		Account: 1,
		Branch:  0,
	}
)

var Net2KeyScope = map[uint32]KeyScope{1: TestnetKeyScope, 2021: MainnetKeyScope}

// KeyScope represents a restricted key scope from the primary root key within
// the HD chain. From the root manager (m/) we can create a nearly arbitrary
// number of ScopedKeyManagers of key derivation path: m/purpose'/coinType'.
// These scoped managers can then me managed indecently, as they house the
// encrypted coinType key and can derive any child keys from there on.
// https://github.com/satoshilabs/slips/blob/master/slip-0044.md
type KeyScope struct {
	// Purpose is the purpose of this key scope. This is the first child of
	// the master HD key.
	Purpose uint32
	// Coin is a value that represents the particular coin which is the
	// child of the purpose key. With this key, any accounts, or other
	// children can be derived at all.
	Coin uint32 // 1-testnet,  2021-mainnet
}

// DerivationPath represents a derivation path from a particular key manager's
// scope.  Each ScopedKeyManager starts key derivation from the end of their
// coinType hardened key: m/purpose'/coinType'. The fields in this struct allow
// further derivation to the next three child levels after the coin type key.
// This restriction is in the spirit of BIP0044 type derivation. We maintain a
// degree of coherency with the standard, but allow arbitrary derivations
// beyond the coinType key. The key derived using this path will be exactly:
// m/purpose'/coinType'/account/branch/index, where purpose' and coinType' are
// bound by the scope of a particular manager.
type DerivationPath struct {
	// Account is the account, or the first immediate child from the scoped
	// manager's hardened coin type key.
	Account uint32

	// Branch is the branch to be derived from the account index above. For
	// BIP0044-like derivation, this is either 0 (external) or 1
	// (internal). However, we allow this value to vary arbitrarily within
	// its size range.
	Branch uint32

	// Index is the final child in the derivation path. This denotes the
	// key index within as a child of the account and branch.
	Index uint32
}

// deriveCoinTypeKey derives the coin type key which can be used to derive the
// extended key for an account according to the hierarchy described by BIP0044
// given the coin type key.
//
// In particular this is the hierarchical deterministic extended key path:
// m/purpose'/<coin type>'
func deriveCoinTypeKey(masterNode *hdkeychain.ExtendedKey,
	scope KeyScope) (*hdkeychain.ExtendedKey, error) {

	// Enforce maximum coin type.
	if scope.Coin > maxCoinType {
		str := "coin type may not exceed " +
			strconv.FormatUint(maxCoinType, 10)
		err := errors.New(str)
		return nil, err
	}

	// The hierarchy described by BIP0043 is:
	//  m/<purpose>'/*
	//
	// This is further extended by BIP0044 to:
	//  m/44'/<coin type>'/<account>'/<branch>/<address index>
	//
	// However, as this is a generic key store for any family for BIP0044
	// standards, we'll use the custom scope to govern our key derivation.
	//
	// The branch is 0 for external addresses and 1 for internal addresses.

	// Derive the purpose key as a child of the master node.
	purpose, err := masterNode.Child(scope.Purpose + hdkeychain.HardenedKeyStart)
	if err != nil {
		return nil, err
	}

	// Derive the coin type key as a child of the purpose key.
	coinTypeKey, err := purpose.Child(scope.Coin + hdkeychain.HardenedKeyStart)
	if err != nil {
		return nil, err
	}

	return coinTypeKey, nil
}

// deriveAccountKey derives the extended key for an account according to the
// hierarchy described by BIP0044 given the master node.
//
// In particular this is the hierarchical deterministic extended key path:
//   m/purpose'/<coin type>'/<account>'
func deriveAccountKey(coinTypeKey *hdkeychain.ExtendedKey,
	account uint32) (*hdkeychain.ExtendedKey, error) {

	// Enforce maximum account number.
	if account > MaxAccountNum {
		str := "account number may not exceed " +
			strconv.FormatUint(MaxAccountNum, 10)
		err := errors.New(str)
		return nil, err
	}

	// Derive the account key as a child of the coin type key.
	return coinTypeKey.Child(account + hdkeychain.HardenedKeyStart)
}

// checkBranchKeys ensures deriving the extended keys for the internal and
// external branches given an account key does not result in an invalid child
// error which means the chosen seed is not usable.  This conforms to the
// hierarchy described by the BIP0044 family so long as the account key is
// already derived accordingly.
//
// In particular this is the hierarchical deterministic extended key path:
//   m/purpose'/<coin type>'/<account>'/<branch>
//
// The branch is 0 for external addresses and 1 for internal addresses.
func checkBranchKeys(acctKey *hdkeychain.ExtendedKey) error {
	// Derive the external branch as the first child of the account key.
	if _, err := acctKey.Child(ExternalBranch); err != nil {
		return err
	}

	// Derive the external branch as the second child of the account key.
	_, err := acctKey.Child(InternalBranch)
	return err
}
