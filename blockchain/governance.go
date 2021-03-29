package blockchain

import (
	"github.com/Sukhavati-Labs/go-miner/version"
	"sync"
)

type GovernAddressClass uint32

const (
	SenateGovernAddress GovernAddressClass = iota // 0
)

type GovernAddress interface {
}

type Govern struct {
	sync.RWMutex
	compatibleVer   *version.Version
	governAddresses map[GovernAddressClass]*GovernAddress
}

func (g *Govern) updateCompatibleVersion(ver *version.Version) {
	g.compatibleVer = ver.Clone()
}

func (g *Govern) UpdateCompatibleVersion(ver *version.Version) {
	g.Lock()
	defer g.Unlock()
	g.updateCompatibleVersion(ver)
}

func (g *Govern) IsCompatibleVersion(ver *version.Version) bool {
	g.Lock()
	defer g.Unlock()
	if g.compatibleVer == nil {
		return true
	}
	if g.compatibleVer.Cmp(ver) > 0 {
		return false
	}
	return true
}
