package protocol

const (
	Symbol                  = "SYJ"
	BaseUnitsPerSYJ         = uint64(100_000_000)
	MaxSupplyBaseUnits      = uint64(72_000_000_000_000_000)
	DefaultBlockRewardUnits = uint64(5_000_000_000)
	TargetBlockTimeSeconds  = int64(30)
	DifficultyInterval      = 10
	MinDifficulty           = 1
	MaxDifficulty           = 32
	MaxAdjustmentFactor     = 4
	AddressPrefix           = "SYJ"
	AddressHashLength       = 40
	CoinbaseSender          = "SYJ-COINBASE-0000000000000000000000000000"
	GenesisMessage          = "SAYANJALI BLOCKCHAIN GENESIS BLOCK - SYJ TOKEN NETWORK"
	GenesisPreviousHash     = "0000000000000000000000000000000000000000000000000000000000000000"
	GenesisTimestamp        = float64(1735689600)
)
