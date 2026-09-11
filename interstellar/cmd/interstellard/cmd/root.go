package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcli "github.com/cometbft/cometbft/libs/cli"

	dbm "github.com/cosmos/cosmos-db"
	cosmosevmcmd "github.com/cosmos/evm/client"
	evmdebug "github.com/cosmos/evm/client/debug"
	"github.com/cosmos/evm/crypto/hd"
	cosmosevmserver "github.com/cosmos/evm/server"
	srvflags "github.com/cosmos/evm/server/flags"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/moziOrg/interstellar-chain/interstellar"
	"github.com/moziOrg/interstellar-chain/interstellar/config"

	"cosmossdk.io/log/v2"
	confixcmd "cosmossdk.io/tools/confix/cmd"
	"github.com/cosmos/cosmos-sdk/store/v2"
	pruningtypes "github.com/cosmos/cosmos-sdk/store/v2/pruning/types"
	snapshottypes "github.com/cosmos/cosmos-sdk/store/v2/snapshots/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	clientcfg "github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/pruning"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/snapshot"
	"github.com/cosmos/cosmos-sdk/codec"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
)

// NewRootCmd creates the Interstellar daemon command tree.
// main function.
func NewRootCmd() *cobra.Command {
	// we "pre"-instantiate the application for getting the injected/configured encoding configuration
	// and the CLI options for the modules
	// add keyring to autocli opts
	tempApp := interstellar.NewApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		true,
		// NewApp configures a process-global EVM chain configuration. The
		// command-tree instance must retain the upstream default so the real
		// application can set the configured Interstellar mainnet or testnet ID.
		simtestutil.AppOptionsMap{srvflags.EVMChainID: evmtypes.DefaultEVMChainID},
	)

	encodingConfig := sdktestutil.TestEncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.GetTxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}
	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithBroadcastMode(flags.FlagBroadcastMode).
		WithHomeDir(config.MustGetDefaultNodeHome()).
		WithViper(""). // In simapp, we don't use any prefix for env variables.
		// Cosmos EVM specific setup
		WithKeyringOptions(hd.EthSecp256k1Option()).
		WithLedgerHasProtobuf(true)

	rootCmd := &cobra.Command{
		Use:   interstellar.AppName,
		Short: "Interstellar Cosmos EVM node",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx = initClientCtx.WithCmdContext(cmd.Context())
			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			initClientCtx, err = clientcfg.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}

			// This needs to go after ReadFromClientConfig, as that function
			// sets the RPC client needed for SIGN_MODE_TEXTUAL. This sign mode
			// is only available if the client is online.
			if !initClientCtx.Offline {
				enabledSignModes := append(tx.DefaultSignModes, signing.SignMode_SIGN_MODE_TEXTUAL) //nolint:gocritic
				txConfigOpts := tx.ConfigOptions{
					EnabledSignModes:           enabledSignModes,
					TextualCoinMetadataQueryFn: txmodule.NewGRPCCoinMetadataQueryFn(initClientCtx),
				}
				txConfig, err := tx.NewTxConfigWithOptions(
					initClientCtx.Codec,
					txConfigOpts,
				)
				if err != nil {
					return err
				}

				initClientCtx = initClientCtx.WithTxConfig(txConfig)
			}

			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			chainID, _ := cmd.Flags().GetString(flags.FlagChainID)
			if chainID == "" {
				chainID = initClientCtx.ChainID
			}
			if chainID != "" && !interstellar.IsSupportedChainID(chainID) {
				return fmt.Errorf("unsupported Interstellar chain ID %q; expected %q, %q, or %q", chainID, interstellar.MainnetChainID, interstellar.TestnetChainID, interstellar.DevnetChainID)
			}
			nodeMode, _ := cmd.Flags().GetString(interstellar.NodeModeFlag)
			if nodeMode == "" {
				nodeMode = interstellar.NodeModeVal
			}
			if !interstellar.IsValidNodeMode(nodeMode) {
				return fmt.Errorf("invalid --%s %q; expected archive, rpc, or val", interstellar.NodeModeFlag, nodeMode)
			}
			if err := applyNodeModeStartFlags(cmd, nodeMode); err != nil {
				return err
			}
			customAppTemplate, customAppConfig := config.InitAppConfig(interstellar.BaseDenom, interstellar.EVMChainIDForChainID(chainID), nodeMode)
			customTMConfig := initCometConfig()

			return sdkserver.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, customTMConfig)
		},
	}
	rootCmd.PersistentFlags().String(interstellar.NodeModeFlag, interstellar.NodeModeVal, "Node profile: archive, rpc, or val")

	initRootCmd(rootCmd, tempApp)

	autoCliOpts := tempApp.AutoCliOpts()
	initClientCtx, _ = clientcfg.ReadFromClientConfig(initClientCtx)
	autoCliOpts.ClientCtx = initClientCtx

	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}

	return rootCmd
}

// applyNodeModeStartFlags applies safe RPC defaults only when the operator did
// not explicitly provide a lower-level start flag. This makes --node-mode rpc
// useful for existing homes as well as newly initialized homes.
func applyNodeModeStartFlags(cmd *cobra.Command, nodeMode string) error {
	if cmd.Name() != "start" || nodeMode != interstellar.NodeModeRPC {
		return nil
	}
	for name, value := range map[string]string{
		"api.enable":              "true",
		"grpc.enable":             "true",
		"json-rpc.enable":         "true",
		"json-rpc.enable-indexer": "true",
		"json-rpc.api":            "eth,net,txpool,web3",
	} {
		flag := cmd.Flags().Lookup(name)
		if flag == nil || flag.Changed {
			continue
		}
		if err := cmd.Flags().Set(name, value); err != nil {
			return fmt.Errorf("set %s node-mode flag %s: %w", nodeMode, name, err)
		}
	}
	return nil
}

// initCometConfig helps to override default CometBFT Config values.
// return cmtcfg.DefaultConfig if no custom configuration is required for the application.
func initCometConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()
	// Cosmos EVM's application mempool requires CometBFT to delegate mempool
	// handling to the ABCI application rather than using the legacy flood pool.
	cfg.Mempool.Type = "app"
	cfg.Consensus.TimeoutCommit = interstellar.TargetBlockInterval

	// these values put a higher strain on node memory
	// cfg.P2P.MaxNumInboundPeers = 100
	// cfg.P2P.MaxNumOutboundPeers = 40

	return cfg
}

func initRootCmd(rootCmd *cobra.Command, evmApp *interstellar.App) {
	cfg := sdk.GetConfig()
	cfg.Seal()

	defaultNodeHome := config.MustGetDefaultNodeHome()
	sdkAppCreator := func(l log.Logger, d dbm.DB, ao servertypes.AppOptions) servertypes.Application {
		return newApp(l, d, ao)
	}
	rootCmd.AddCommand(
		newInitCmd(evmApp, defaultNodeHome),
		genutilcli.Commands(evmApp.TxConfig(), evmApp.BasicModuleManager, defaultNodeHome),
		cmtcli.NewCompletionCmd(rootCmd, true),
		evmdebug.Cmd(),
		confixcmd.ConfigCommand(),
		pruning.Cmd(sdkAppCreator, defaultNodeHome),
		snapshot.Cmd(sdkAppCreator),
		newRepairStateCmd(defaultNodeHome),
		NewTestnetCmd(evmApp.BasicModuleManager, banktypes.GenesisBalancesIterator{}, appCreator{}),
	)

	// add Cosmos EVM' flavored TM commands to start server, etc.
	cosmosevmserver.AddCommands(
		rootCmd,
		cosmosevmserver.NewDefaultStartOptions(newApp, defaultNodeHome),
		appExport,
		addModuleInitFlags,
	)

	// add Cosmos EVM key commands
	rootCmd.AddCommand(
		cosmosevmcmd.KeyCommands(defaultNodeHome, true),
	)

	// add keybase, auxiliary RPC, query, genesis, and tx child commands
	rootCmd.AddCommand(
		sdkserver.StatusCommand(),
		queryCommand(),
		txCommand(),
	)

	// add general tx flags to the root command
	var err error
	_, err = srvflags.AddTxFlags(rootCmd)
	if err != nil {
		panic(err)
	}
}

// newInitCmd wraps the SDK command so every newly initialized home starts with
// the Interstellar protocol denominations and EVM genesis parameters.
func newInitCmd(app *interstellar.App, defaultNodeHome string) *cobra.Command {
	cmd := genutilcli.InitCmd(app.BasicModuleManager, defaultNodeHome)
	defaultDenom := cmd.Flags().Lookup(genutilcli.FlagDefaultBondDenom)
	defaultDenom.DefValue = interstellar.BaseDenom
	defaultDenom.Usage = "genesis file default denomination (default ahuge)"
	if err := defaultDenom.Value.Set(interstellar.BaseDenom); err != nil {
		panic(err)
	}

	sdkRunE := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		denom, err := cmd.Flags().GetString(genutilcli.FlagDefaultBondDenom)
		if err != nil {
			return err
		}
		if denom != interstellar.BaseDenom {
			return errors.New("Interstellar init requires --default-denom ahuge")
		}
		if err := sdkRunE(cmd, args); err != nil {
			return err
		}

		home, err := cmd.Flags().GetString(flags.FlagHome)
		if err != nil {
			return err
		}
		genesisFile := filepath.Join(home, "config", "genesis.json")
		if err := applyInterstellarGenesisFile(app.AppCodec(), genesisFile); err != nil {
			return err
		}
		cmd.PrintErrf("Interstellar genesis defaults applied: %s\n", genesisFile)
		return nil
	}

	return cmd
}

func applyInterstellarGenesisFile(cdc codec.Codec, genesisFile string) error {
	appGenesis, err := genutiltypes.AppGenesisFromFile(genesisFile)
	if err != nil {
		return err
	}

	genesisState := interstellar.GenesisState{}
	if err := json.Unmarshal(appGenesis.AppState, &genesisState); err != nil {
		return err
	}

	appGenesis.AppName = interstellar.AppName
	if appGenesis.Consensus == nil {
		appGenesis.Consensus = &genutiltypes.ConsensusGenesis{}
	}
	appGenesis.Consensus.Params = interstellar.NewConsensusParams()
	appGenesis.AppState, err = json.MarshalIndent(interstellar.ApplyGenesisDefaults(cdc, genesisState), "", "  ")
	if err != nil {
		return err
	}

	return appGenesis.SaveAs(genesisFile)
}

func addModuleInitFlags(_ *cobra.Command) {}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.QueryEventForTxCmd(),
		rpc.ValidatorCommand(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
		sdkserver.QueryBlockCmd(),
		sdkserver.QueryBlockResultsCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		authcmd.GetSimulateCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

// newApp creates the application
func newApp(
	logger log.Logger,
	db dbm.DB,
	appOpts servertypes.AppOptions,
) cosmosevmserver.Application {
	var cache storetypes.MultiStorePersistentCache

	if cast.ToBool(appOpts.Get(sdkserver.FlagInterBlockCache)) {
		cache = store.NewCommitKVStoreCacheManager()
	}

	pruningOpts, err := sdkserver.GetPruningOptionsFromFlags(appOpts)
	if err != nil {
		panic(err)
	}
	if cast.ToString(appOpts.Get(interstellar.NodeModeFlag)) == interstellar.NodeModeArchive {
		pruningOpts = pruningtypes.NewPruningOptionsFromString(pruningtypes.PruningOptionNothing)
	}

	// get the chain id
	chainID, err := getChainIDFromOpts(appOpts)
	if err != nil {
		panic(err)
	}
	if err := validateEVMChainID(appOpts, chainID); err != nil {
		panic(err)
	}

	snapshotStore, err := sdkserver.GetSnapshotStore(appOpts)
	if err != nil {
		panic(err)
	}

	snapshotOptions := snapshottypes.NewSnapshotOptions(
		cast.ToUint64(appOpts.Get(sdkserver.FlagStateSyncSnapshotInterval)),
		cast.ToUint32(appOpts.Get(sdkserver.FlagStateSyncSnapshotKeepRecent)),
	)

	baseappOptions := []func(*baseapp.BaseApp){
		baseapp.SetPruning(pruningOpts),
		baseapp.SetMinGasPrices(cast.ToString(appOpts.Get(sdkserver.FlagMinGasPrices))),
		baseapp.SetQueryGasLimit(cast.ToUint64(appOpts.Get(sdkserver.FlagQueryGasLimit))),
		baseapp.SetHaltHeight(cast.ToUint64(appOpts.Get(sdkserver.FlagHaltHeight))),
		baseapp.SetHaltTime(cast.ToUint64(appOpts.Get(sdkserver.FlagHaltTime))),
		baseapp.SetMinRetainBlocks(cast.ToUint64(appOpts.Get(sdkserver.FlagMinRetainBlocks))),
		baseapp.SetInterBlockCache(cache),
		baseapp.SetTrace(cast.ToBool(appOpts.Get(sdkserver.FlagTrace))),
		baseapp.SetIndexEvents(cast.ToStringSlice(appOpts.Get(sdkserver.FlagIndexEvents))),
		baseapp.SetSnapshot(snapshotStore, snapshotOptions),
		baseapp.SetIAVLCacheSize(cast.ToInt(appOpts.Get(sdkserver.FlagIAVLCacheSize))),
		baseapp.SetIAVLDisableFastNode(cast.ToBool(appOpts.Get(sdkserver.FlagDisableIAVLFastNode))),
		baseapp.SetChainID(chainID),
	}

	return interstellar.NewApp(
		logger, db, true,
		appOpts,
		baseappOptions...,
	)
}

// appExport creates a new application (optionally at a given height) and exports state.
func appExport(
	logger log.Logger,
	db dbm.DB,
	height int64,
	forZeroHeight bool,
	jailAllowedAddrs []string,
	appOpts servertypes.AppOptions,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	var exampleApp *interstellar.App

	// this check is necessary as we use the flag in x/upgrade.
	// we can exit more gracefully by checking the flag here.
	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, errors.New("application home not set")
	}

	viperAppOpts, ok := appOpts.(*viper.Viper)
	if !ok {
		return servertypes.ExportedApp{}, errors.New("appOpts is not viper.Viper")
	}

	// overwrite the FlagInvCheckPeriod
	viperAppOpts.Set(sdkserver.FlagInvCheckPeriod, 1)
	appOpts = viperAppOpts

	// get the chain id
	chainID, err := getChainIDFromOpts(appOpts)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}
	if err := validateEVMChainID(appOpts, chainID); err != nil {
		return servertypes.ExportedApp{}, err
	}

	if height != -1 {
		exampleApp = interstellar.NewApp(logger, db, false, appOpts, baseapp.SetChainID(chainID))

		if err := exampleApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		exampleApp = interstellar.NewApp(logger, db, true, appOpts, baseapp.SetChainID(chainID))
	}

	return exampleApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}

// The genesis chain ID is authoritative; a command-line override must agree.
func getChainIDFromOpts(appOpts servertypes.AppOptions) (chainID string, err error) {
	genesisFile := filepath.Join(cast.ToString(appOpts.Get(flags.FlagHome)), "config", "genesis.json")
	appGenesis, err := genutiltypes.AppGenesisFromFile(genesisFile)
	if err != nil {
		return "", err
	}
	chainID = appGenesis.ChainID
	if requested := cast.ToString(appOpts.Get(flags.FlagChainID)); requested != "" && requested != chainID {
		return "", fmt.Errorf("Cosmos chain-id mismatch: configured %q, genesis %q", requested, chainID)
	}

	return chainID, err
}

// validateEVMChainID checks the effective config, including flag overrides.
// EVM chain ID is a consensus input, not a per-node preference.
func validateEVMChainID(appOpts servertypes.AppOptions, chainID string) error {
	if !interstellar.IsSupportedChainID(chainID) {
		return fmt.Errorf("unsupported Interstellar chain ID %q", chainID)
	}
	actual, err := cast.ToUint64E(appOpts.Get(srvflags.EVMChainID))
	expected := interstellar.EVMChainIDForChainID(chainID)
	if err != nil || actual != expected {
		return fmt.Errorf("EVM chain ID mismatch for Cosmos chain-id %q: got %v, expected %d; check %s and --%s", chainID, appOpts.Get(srvflags.EVMChainID), expected, filepath.Join(cast.ToString(appOpts.Get(flags.FlagHome)), "config", "app.toml"), srvflags.EVMChainID)
	}
	return nil
}
