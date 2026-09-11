package interstellar

import "time"

const (
	// AppName is both the daemon name and the ABCI application name.
	AppName = "interstellard"

	// BaseDenom is the 18-decimal integer denomination used by the SDK and EVM.
	BaseDenom = "ahuge"
	// DisplayDenom and DenomSymbol are wallet-facing denominations for the native asset.
	DisplayDenom  = "huge"
	DenomSymbol   = "HUGE"
	DenomExponent = 18

	MainnetChainID = "intl-main"
	TestnetChainID = "intl-testnet-1"
	DevnetChainID  = "intl-dev-1"

	MainnetEVMChainID uint64 = 1677
	TestnetEVMChainID uint64 = 1678
	DevnetEVMChainID  uint64 = 1679

	Bech32Prefix = "hg"

	// DefaultMinGasPriceWei is the consensus fee-market floor and the default
	// local transaction admission floor. It is one gwei expressed in ahuge.
	DefaultMinGasPriceWei int64 = 1_000_000_000

	// DefaultBlockGasLimit is the launch block capacity. It lives in the
	// Cosmos consensus parameters and can be changed later through governance.
	DefaultBlockGasLimit int64 = 100_000_000
	// DefaultMaxValidators is the initial active-validator-set limit. It is a
	// staking parameter in genesis and can subsequently be updated by governance.
	DefaultMaxValidators uint32 = 5
	// TargetBlockInterval is the healthy-round commit interval for new nodes.
	TargetBlockInterval time.Duration = 3 * time.Second

	NodeModeFlag    = "node-mode"
	NodeModeArchive = "archive"
	NodeModeRPC     = "rpc"
	NodeModeVal     = "val"
)

// EVMChainIDForChainID maps the supported Interstellar network identifiers to their
// EIP-155 replay-protection IDs. Command entry points validate the chain ID
// before invoking this helper, so an unsupported network cannot silently use
// the mainnet replay-protection domain.
func EVMChainIDForChainID(chainID string) uint64 {
	switch chainID {
	case MainnetChainID:
		return MainnetEVMChainID
	case TestnetChainID:
		return TestnetEVMChainID
	case DevnetChainID:
		return DevnetEVMChainID
	default:
		return MainnetEVMChainID
	}
}

func IsSupportedChainID(chainID string) bool {
	return chainID == MainnetChainID || chainID == TestnetChainID || chainID == DevnetChainID
}

func IsValidNodeMode(mode string) bool {
	return mode == NodeModeArchive || mode == NodeModeRPC || mode == NodeModeVal
}
