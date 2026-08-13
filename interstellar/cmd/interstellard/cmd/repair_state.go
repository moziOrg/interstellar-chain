package cmd

import (
	"fmt"
	"path/filepath"

	cmtcmd "github.com/cometbft/cometbft/cmd/cometbft/commands"
	"github.com/spf13/cobra"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
)

// newRepairStateCmd repairs a committed state-machine mismatch by rolling the
// application and CometBFT stores back together. It deliberately does not try
// to salvage a physically corrupted database: that must be restored from a
// verified snapshot or synchronized from a healthy peer.
func newRepairStateCmd(defaultNodeHome string) *cobra.Command {
	var (
		hard  bool
		count uint
		yes   bool
	)

	cmd := &cobra.Command{
		Use:   "repair-state",
		Short: "Repair state-machine data by rolling back matched state versions",
		Long: `Repair state-machine data when CometBFT and the application have a
known incorrect or mismatched latest committed state. The node must be stopped.

This command rolls back CometBFT state and the application multistore together.
It does not repair checksum errors, missing SST files, disk corruption, or a
corrupted blockstore. Back up the complete home directory first.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				return fmt.Errorf("refusing to modify state without --yes; stop the node and create a full backup first")
			}
			if count == 0 {
				return fmt.Errorf("--rollback-count must be at least 1")
			}

			ctx := sdkserver.GetServerContextFromCmd(cmd)
			db, err := dbm.NewDB(
				"application",
				sdkserver.GetAppDBBackend(ctx.Viper),
				filepath.Join(ctx.Config.RootDir, "data"),
			)
			if err != nil {
				return fmt.Errorf("open application database: %w", err)
			}
			defer func() { _ = db.Close() }()

			app := newApp(ctx.Logger, db, ctx.Viper)
			for i := uint(0); i < count; i++ {
				height, hash, err := cmtcmd.RollbackState(ctx.Config, hard)
				if err != nil {
					return fmt.Errorf("rollback CometBFT state at step %d: %w", i+1, err)
				}
				if err := app.CommitMultiStore().RollbackToVersion(height); err != nil {
					return fmt.Errorf("rollback application state to height %d: %w", height, err)
				}
				cmd.Printf("repaired state to height %d, app hash %X\n", height, hash)
			}
			return nil
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "Directory for config and data")
	cmd.Flags().BoolVar(&hard, "hard", false, "Also remove each rolled-back block")
	cmd.Flags().UintVar(&count, "rollback-count", 1, "Number of latest committed heights to roll back")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm that the node is stopped and the complete home directory is backed up")
	return cmd
}
