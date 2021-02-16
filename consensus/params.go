package consensus

const (
	SukhavatiPerSkt uint64 = 100000000
	// 1 skt, 1 day old, a tx of 250 bytes
	MinHighPriority          = 1e8 * 1280.0 / 250
	MaxNewBlockChSize        = 1024
	DefaultBlockPrioritySize = 50000 // bytes  40MB

	// staking tx
	MaxStakingRewardNum                = 30
	defaultStakingTxRewardStart uint64 = 24

	defaultCoinbaseMaturity    uint64 = 1000
	defaultTransactionMaturity uint64 = 1

	defaultMaxSkt        uint64 = 618033989 //206438400
	defaultMinRelayTxFee uint64 = 10000

	defaultSubsidyHalvingInterval uint64 = 13440
	defaultBaseSubsidy            uint64 = 128 * SukhavatiPerSkt
	defaultMinHalvedSubsidy       uint64 = 6250000

	defaultMinFrozenPeriod uint64 = 61440
	defaultMinStakingValue uint64 = 2048 * SukhavatiPerSkt

	Ip1Activation  uint64 = 694000
	MaxValidPeriod        = defaultMinFrozenPeriod * 24 // 1474560
	// 40s height + 1  1day 24 * 60 * 60 = 86400s  86400s % 40s = 2160
	// release staking pool  1/200
	StakingPoolRewardProportionalDenominator = 200
	// 90 day --> height
	StakingPoolRewardStartHeight = 194400
	StakingPoolRewardEpoch       = 2160
	// 5.838% --> 94.162%   --> 94162/100000
	CoinbaseSubsidyAttenuation            = 94162
	CoinbaseSubsidyAttenuationDenominator = 100000
)

var (
	// CoinbaseMaturity is the number of blocks required before newly
	// mined coins can be spent
	CoinbaseMaturity = defaultCoinbaseMaturity

	// TransactionMaturity is the number of blocks required before newly
	// binding tx get reward
	TransactionMaturity = defaultTransactionMaturity

	// MaxSkt the maximum Sukhavati amount
	MaxSkt = defaultMaxSkt

	SubsidyHalvingInterval = defaultSubsidyHalvingInterval
	// BaseSubsidy is the original subsidy Sukhavati for mined blocks.  This
	// value is halved every SubsidyHalvingInterval blocks.
	BaseSubsidy = defaultBaseSubsidy
	// MinHalvedSubsidy is the minimum subsidy Sukhavati for mined blocks.
	MinHalvedSubsidy = defaultMinHalvedSubsidy

	// MinRelayTxFee minimum relay fee in Sukhavati
	MinRelayTxFee = defaultMinRelayTxFee

	//Min Frozen Period in a StakingScriptHash output
	MinFrozenPeriod = defaultMinFrozenPeriod
	//MinStakingValue minimum StakingScriptHash output in Sukhavati
	MinStakingValue = defaultMinStakingValue

	StakingTxRewardStart = defaultStakingTxRewardStart

	TestStakingPoolWitness = []byte("skt_pool")
)
