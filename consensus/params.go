package consensus

const (
	SukhavatiPerSkt uint64 = 100000000
	// MinHighPriority 1 skt, 1 day old, a tx of 250 bytes
	MinHighPriority          = 1e8 * 1280.0 / 250
	MaxNewBlockChSize        = 1024
	DefaultBlockPrioritySize = 50000 // bytes  40MB

	// MaxStakingRewardNum staking tx
	MaxStakingRewardNum                  = 100
	defaultStakingTxRewardStart   uint64 = 24
	defaultGovernanceTxTakeEffect uint64 = 24
	defaultCoinbaseMaturity       uint64 = 10
	defaultTransactionMaturity    uint64 = 1

	// +-----------+--------------------+
	// | mining    | 381945005          |
	// +-----------+--------------------+
	// | investor  | 92705098           |
	// +-----------+--------------------+
	// | ecology   | 38318107           |
	// +-----------+--------------------+
	// | team      | 61803399           |
	// +-----------+--------------------+
	// | foundation| 43262379           |
	// +-----------+--------------------+
	defaultMiningMaxSkt     uint64 = 381945005 //38194500500000000  87B1B222AD0D00
	defaultInvestorMaxSkt   uint64 = 92705098  //9270509800000000   20EF7AC3840A00
	defaultEcologyMaxSkt    uint64 = 38318107  //3831810700000000   D9D02F39EBB00
	defaultTeamMaxSkt       uint64 = 61803399  //6180339900000000   15F4FC8454A700
	defaultFoundationMaxSkt uint64 = 43262379  //4326237900000000   F5EB0C13E4B00
	defaultMaxSkt                  = defaultMiningMaxSkt + defaultInvestorMaxSkt + defaultEcologyMaxSkt + defaultTeamMaxSkt + defaultFoundationMaxSkt
	defaultMinRelayTxFee    uint64 = 10000

	defaultSubsidyHalvingInterval uint64 = 13440
	defaultBaseSubsidy            uint64 = 128 * SukhavatiPerSkt
	defaultMinHalvedSubsidy       uint64 = 6250000

	DayPeriod uint64 = 1920
	// DaySeconds The number of seconds in a day
	DaySeconds             uint64 = 86400
	defaultMinFrozenPeriod uint64 = 61440
	defaultMinStakingValue uint64 = 2048 * SukhavatiPerSkt
	// StakingPoolAwardActivation after 90 days  activation ,and dev only 1
	StakingPoolAwardActivation uint64 = 1
	MaxValidPeriod                    = defaultMinFrozenPeriod * 24 // 1474560
	// StakingPoolRewardProportionalDenominator 40s height + 1  1day 24 * 60 * 60 = 86400s  86400s % 40s = 2160
	// release staking pool  1/200
	StakingPoolRewardProportionalDenominator = 200
	// StakingPoolMergeEpoch staking pool merge  epoch
	StakingPoolMergeEpoch = 100
	StakingPoolAwardStart = 2
	// StakingPoolRewardStartHeight 90 day --> height
	StakingPoolRewardStartHeight = 194400
	StakingPoolRewardEpoch       = 2160
	// CoinbaseSubsidyAttenuation 5.838% --> 94.162%   --> 94162/100000
	CoinbaseSubsidyAttenuation            = 94162
	CoinbaseSubsidyAttenuationDenominator = 100000
	governAddress                         = "sk1qqpthgpk7yqjmenj6fe3klp9d98ay02e5sc4k8avk8985zty3spvdqdfdy3y"
	StakingPoolType                       = uint16(1)
	// BindingTxFrozenPeriod default binding frozen period
	BindingTxFrozenPeriod  = 90 * DayPeriod
	AwardingTxFrozenPeriod = 1 // DayPeriod
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

	// MinFrozenPeriod Min Frozen Period in a StakingScriptHash output
	MinFrozenPeriod = defaultMinFrozenPeriod
	// MinAwardFrozenPeriod min pooling tx award frozen period
	MinAwardFrozenPeriod = 1
	//MinStakingValue minimum StakingScriptHash output in Sukhavati
	MinStakingValue = defaultMinStakingValue
	// StakingTxRewardStart staking tx
	StakingTxRewardStart = defaultStakingTxRewardStart
	BindingRequiredSkt   = map[int]float64{
		24: 0.006144,
		26: 0.026624,
		28: 0.112,
		30: 0.48,
		32: 2.048,
		34: 8.704,
		36: 36.864,
		38: 152,
		40: 640,
	}
	// StakingFrozenPeriodWeight day --> weight * 10000
	StakingFrozenPeriodWeight = map[uint64]uint64{
		55:  10000,
		144: 16180,
		377: 26180,
	}
)
