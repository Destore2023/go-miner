package wallet

import (
	"path/filepath"
	"strings"
)

const (
	DBDirName = "keystore"
)

type PoCWalletConfig struct {
	dbType string
	dbPath string
}

func (c *PoCWalletConfig) DBPath() string {
	return c.dbPath
}

func (c PoCWalletConfig) DBType() string {
	return c.dbType
}

func NewPocWalletConfig(root string, dbType string) *PoCWalletConfig {
	if !strings.HasSuffix(root, DBDirName) {
		root = filepath.Join(root, DBDirName)
	}
	return &PoCWalletConfig{
		dbPath: root,
		dbType: dbType,
	}
}
