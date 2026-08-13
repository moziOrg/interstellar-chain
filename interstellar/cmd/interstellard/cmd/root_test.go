package cmd

import (
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/moziOrg/interstellar-chain/interstellar"
)

func TestApplyNodeModeStartFlags(t *testing.T) {
	command := &cobra.Command{Use: "start"}
	command.Flags().Bool("api.enable", false, "")
	command.Flags().Bool("grpc.enable", false, "")
	command.Flags().Bool("json-rpc.enable", false, "")
	command.Flags().Bool("json-rpc.enable-indexer", false, "")
	command.Flags().StringSlice("json-rpc.api", nil, "")

	if err := applyNodeModeStartFlags(command, "rpc"); err != nil {
		t.Fatalf("apply rpc mode: %v", err)
	}
	for _, name := range []string{"api.enable", "grpc.enable", "json-rpc.enable", "json-rpc.enable-indexer"} {
		value, err := command.Flags().GetBool(name)
		if err != nil || !value {
			t.Fatalf("expected %s to be enabled", name)
		}
	}

	if err := command.Flags().Set("json-rpc.enable", "false"); err != nil {
		t.Fatal(err)
	}
	if err := applyNodeModeStartFlags(command, "rpc"); err != nil {
		t.Fatalf("reapply rpc mode: %v", err)
	}
	value, err := command.Flags().GetBool("json-rpc.enable")
	if err != nil || value {
		t.Fatal("explicit operator flag must take precedence over node mode")
	}
}

func TestInitCometConfigUsesInterstellarConsensusDefaults(t *testing.T) {
	config := initCometConfig()
	if config.Mempool.Type != "app" {
		t.Fatalf("expected app mempool, got %q", config.Mempool.Type)
	}
	if config.Consensus.TimeoutCommit != 3*time.Second {
		t.Fatalf("expected 3s commit interval, got %s", config.Consensus.TimeoutCommit)
	}
	if config.Consensus.TimeoutCommit != interstellar.TargetBlockInterval {
		t.Fatal("commit interval must match Interstellar protocol constant")
	}
}
