package config

import (
	"testing"

	"github.com/cosmos/evm/crypto/hd"
)

func TestInterstellarAddressConfigurationConstants(t *testing.T) {
	if Bech32Prefix != "hg" {
		t.Fatalf("unexpected account prefix: %s", Bech32Prefix)
	}
	if Bech32PrefixValAddr != "hgvaloper" {
		t.Fatalf("unexpected validator prefix: %s", Bech32PrefixValAddr)
	}
	if Bech32PrefixConsAddr != "hgvalcons" {
		t.Fatalf("unexpected consensus prefix: %s", Bech32PrefixConsAddr)
	}
	if hd.Bip44CoinType != 60 {
		t.Fatalf("expected Ethereum coin type 60, got %d", hd.Bip44CoinType)
	}
}
