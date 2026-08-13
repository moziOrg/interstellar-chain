package config

import (
	"strconv"

	clienthelpers "cosmossdk.io/client/v2/helpers"
	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"
)

const defaultMinGasPriceWei int64 = 1_000_000_000

func MustGetDefaultNodeHome() string {
	defaultNodeHome, err := clienthelpers.GetNodeHomeDirectory(".interstellard")
	if err != nil {
		panic(err)
	}
	return defaultNodeHome
}

// InitAppConfig helps to override default appConfig template and configs.
// return "", nil if no custom configuration is required for the application.
func InitAppConfig(denom string, evmChainID uint64, nodeMode string) (string, interface{}) {
	// Optionally allow the chain developer to overwrite the SDK's default
	// server config.
	srvCfg := serverconfig.DefaultConfig()
	// The SDK's default minimum gas price is set to "" (empty value) inside
	// app.toml. If left empty by validators, the node will halt on startup.
	// However, the chain developer can set a default app.toml value for their
	// validators here.
	//
	// In summary:
	// - if you leave srvCfg.MinGasPrices = "", all validators MUST tweak their
	//   own app.toml config,
	// - if you set srvCfg.MinGasPrices non-empty, validators CAN tweak their
	//   own app.toml to override, or use this default value.
	//
	// Keep the node's local admission policy aligned with the non-zero
	// consensus fee-market floor used by a newly created Interstellar network.
	srvCfg.MinGasPrices = strconv.FormatInt(defaultMinGasPriceWei, 10) + denom

	evmCfg := cosmosevmserverconfig.DefaultEVMConfig()
	evmCfg.EVMChainID = evmChainID

	customAppConfig := EVMAppConfig{
		Config:  *srvCfg,
		EVM:     *evmCfg,
		JSONRPC: *cosmosevmserverconfig.DefaultJSONRPCConfig(),
		TLS:     *cosmosevmserverconfig.DefaultTLSConfig(),
	}
	// All supplied profiles serve a bounded set of snapshots. Snapshot interval
	// is a local operational setting and does not affect consensus.
	customAppConfig.StateSync.SnapshotInterval = 10_000
	customAppConfig.StateSync.SnapshotKeepRecent = 3

	switch nodeMode {
	case "val":
		// Validators retain enough state to serve as dependable state-sync
		// sources while avoiding archive-node disk growth. CometBFT block
		// retention is intentionally left at zero: it is constrained by the
		// unbonding safety window and must be changed as an operator policy.
		customAppConfig.Pruning = "custom"
		customAppConfig.PruningKeepRecent = "362880"
		customAppConfig.PruningInterval = "10"
	case "archive":
		// Keep every committed application-state version. CometBFT block
		// retention and transaction indexing remain independently configurable.
		customAppConfig.Pruning = "nothing"
	case "rpc":
		// RPC nodes retain a bounded query window and EVM index data. They are
		// not archival providers; deploy a separate archive profile for that.
		customAppConfig.Pruning = "custom"
		customAppConfig.PruningKeepRecent = "362880"
		customAppConfig.PruningInterval = "10"
		customAppConfig.API.Enable = true
		customAppConfig.GRPC.Enable = true
		customAppConfig.JSONRPC.Enable = true
		customAppConfig.JSONRPC.EnableIndexer = true
		customAppConfig.JSONRPC.API = []string{"eth", "net", "txpool", "web3"}
	}

	return EVMAppTemplate, customAppConfig
}

type EVMAppConfig struct {
	serverconfig.Config

	EVM     cosmosevmserverconfig.EVMConfig
	JSONRPC cosmosevmserverconfig.JSONRPCConfig
	TLS     cosmosevmserverconfig.TLSConfig
}

const EVMAppTemplate = serverconfig.DefaultConfigTemplate + cosmosevmserverconfig.DefaultEVMConfigTemplate
