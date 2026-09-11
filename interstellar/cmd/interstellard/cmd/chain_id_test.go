package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/testutil/sims"
	srvflags "github.com/cosmos/evm/server/flags"
	"github.com/stretchr/testify/require"
)

func TestValidateEVMChainID(t *testing.T) {
	for _, tc := range []struct {
		chain string
		evm   interface{}
		valid bool
	}{
		{"intl-main", uint64(1677), true},
		{"intl-testnet-1", "1678", true},
		{"intl-dev-1", 1679, true},
		{"intl-testnet-1", 1677, false},
		{"intl-testnet-1", nil, false},
		{"intl-testnet-1", "invalid", false},
		{"other-chain", 1677, false},
	} {
		opts := sims.AppOptionsMap{srvflags.EVMChainID: tc.evm, flags.FlagHome: t.TempDir()}
		err := validateEVMChainID(opts, tc.chain)
		if tc.valid {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func TestChainIDMustMatchGenesis(t *testing.T) {
	homeDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, "config"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, "config", "genesis.json"), []byte(`{"chain_id":"intl-testnet-1"}`), 0o600))
	for _, requested := range []string{"", "intl-testnet-1", "intl-main"} {
		got, err := getChainIDFromOpts(sims.AppOptionsMap{flags.FlagHome: homeDir, flags.FlagChainID: requested})
		if requested == "intl-main" {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, "intl-testnet-1", got)
		}
	}
}
