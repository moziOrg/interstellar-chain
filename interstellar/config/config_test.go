package config

import "testing"

func TestInitAppConfigProfiles(t *testing.T) {
	for _, mode := range []string{"val", "rpc"} {
		_, value := InitAppConfig("ahuge", 1678, mode)
		cfg := value.(EVMAppConfig)
		if cfg.Pruning != "custom" || cfg.PruningKeepRecent != "362880" || cfg.PruningInterval != "10" {
			t.Fatalf("unexpected %s pruning profile: %+v", mode, cfg.Config)
		}
		if cfg.StateSync.SnapshotInterval != 10_000 || cfg.StateSync.SnapshotKeepRecent != 3 {
			t.Fatalf("unexpected %s snapshot profile: %+v", mode, cfg.StateSync)
		}
	}

	_, value := InitAppConfig("ahuge", 1677, "archive")
	archive := value.(EVMAppConfig)
	if archive.Pruning != "nothing" {
		t.Fatalf("expected archive pruning nothing, got %q", archive.Pruning)
	}
	if archive.MinGasPrices != "1000000000ahuge" {
		t.Fatalf("unexpected minimum gas price %q", archive.MinGasPrices)
	}
	if archive.StateSync.SnapshotInterval != 10_000 || archive.StateSync.SnapshotKeepRecent != 3 {
		t.Fatalf("unexpected archive snapshot profile: %+v", archive.StateSync)
	}

}
