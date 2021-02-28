package wallet

import "path/filepath"

type PoCWalletConfig struct {
	dataDir string
	dbType  string
	dbPath  string
}

func (c *PoCWalletConfig) DBPath() string {
	if c.dbPath == "" {
		c.dbPath = filepath.Join(c.dataDir, "keystore")
	}
	return c.dbPath
}

func (c PoCWalletConfig) DBType() string {
	return c.dbType
}

func NewPocWalletConfig(root string, dbType string) *PoCWalletConfig {
	return &PoCWalletConfig{
		dataDir: root,
		dbType:  dbType,
	}
}
