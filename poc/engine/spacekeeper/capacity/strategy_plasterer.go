package capacity

import (
	"github.com/Sukhavati-Labs/go-miner/chainutil/service"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/poc/engine"
	"github.com/Sukhavati-Labs/go-miner/poc/engine/spacekeeper"
	"github.com/panjf2000/ants"
)

const TypeSpaceKeeperPlasterer = "spacekeeper.plasterer"

func NewSpaceKeeperPlasterer(args ...interface{}) (spacekeeper.SpaceKeeper, error) {
	cfg, poCWallet, err := parseArgs(args...)
	if err != nil {
		return nil, err
	}
	workerPool, err := ants.NewPoolPreMalloc(maxPoolWorker)
	if err != nil {
		return nil, err
	}
	sk := &SpaceKeeper{
		allowGenerateNewSpace: false,
		dbDirs:                cfg.Miner.ProofDir,
		dbType:                regPocDBV1,
		wallet:                poCWallet,
		workSpaceIndex:        make([]*WorkSpaceMap, 0),
		workSpacePaths:        make(map[string]*WorkSpacePath),
		workSpaceList:         make([]*WorkSpace, 0),
		queue:                 newPlotterQueue(),
		newQueuedWorkSpaceCh:  make(chan *queuedWorkSpace, plotterMaxChanSize),
		workerPool:            workerPool,
		fileWatcher:           func() {},
	}
	sk.BaseService = service.NewBaseService(sk, TypeSpaceKeeperV1)
	sk.generateInitialIndex = func() error { return generateInitialIndex(sk, regPocDBV1, regPocDBV1, suffixPocDBV1) }

	if err = sk.generateInitialIndex(); err != nil {
		return nil, err
	}

	if cfg.Miner.PrivatePassword != "" {
		if err = poCWallet.Unlock([]byte(cfg.Miner.PrivatePassword)); err != nil {
			return nil, err
		}
		var wsiList []engine.WorkSpaceInfo
		var configureMethod = "ConfigureByFlags"
		wsiList, err = sk.ConfigureByFlags(engine.SFAll, cfg.Miner.Plot, cfg.Miner.Generate)
		if err != nil {
			return nil, err
		}
		logging.CPrint(logging.DEBUG, "try configure spaceKeeper", logging.LogFormat{"content": wsiList, "err": err, "method": configureMethod})
	}

	return sk, nil
}

func init() {
	spacekeeper.AddSpaceKeeperBackend(spacekeeper.Backend{
		Typ:            TypeSpaceKeeperPlasterer,
		NewSpaceKeeper: NewSpaceKeeperPlasterer,
	})
}
