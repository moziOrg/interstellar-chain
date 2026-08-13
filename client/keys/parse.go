package keys

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ParseKeyStringCommand converts the two address encodings used by MOCIUS.
// A Bech32 hg1 address and an EVM 0x address encode the same 20 account bytes.
func ParseKeyStringCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "parse <address>",
		Short: "Convert a MOCIUS Bech32 address and an EVM address",
		Long: "Convert between an hg1 MOCIUS account address and its corresponding " +
			"0x EVM address. Both encodings represent the same 20-byte account.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			account, err := parseAccountAddress(args[0])
			if err != nil {
				return err
			}

			evmAddress := common.BytesToAddress(account.Bytes())
			cmd.Printf("bech32: %s\n", account.String())
			cmd.Printf("evm: %s\n", evmAddress.Hex())
			cmd.Printf("evm_lowercase: %s\n", strings.ToLower(evmAddress.Hex()))
			cmd.Printf("bytes: %X\n", account.Bytes())
			return nil
		},
	}
}

func parseAccountAddress(input string) (sdk.AccAddress, error) {
	input = strings.TrimSpace(input)
	if common.IsHexAddress(input) {
		return sdk.AccAddress(common.HexToAddress(input).Bytes()), nil
	}

	account, err := sdk.AccAddressFromBech32(input)
	if err != nil {
		return nil, fmt.Errorf("invalid MOCIUS Bech32 or EVM address %q: %w", input, err)
	}
	return account, nil
}
