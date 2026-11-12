package genesisconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
)

func TestAuditCharacterizationProductionArtifactIsNotNodeGenesisState(t *testing.T) {
	cfg := Config{
		NetworkName:         tokenomics.Phase7NetworkName,
		Environment:         Production,
		GenesisStateVersion: "1",
		TokenomicsVersion:   "1",
		Allocations: []Allocation{
			{Category: "presale", Address: "SYJc7181103b1e2cc71112a1517e1fe6c11bf1420a9", AmountBaseUnits: 7_200_000_000_000_000},
			{Category: "treasury", Address: "SYJ3bc4761687ea7f2513642fb94eb44a1e1d3dc2d4", AmountBaseUnits: 5_760_000_000_000_000},
			{Category: "ecosystem", Address: "SYJ69cdd15b3450cecd1713bba9667af43f73b468ec", AmountBaseUnits: 3_600_000_000_000_000},
			{Category: "liquidity", Address: "SYJ20f05c8b84682cac983a0508689257f51ce1c73b", AmountBaseUnits: 4_320_000_000_000_000},
			{Category: "team", Address: "SYJec01044d9a6b32e7afdfbc34e845b3056308b3d9", AmountBaseUnits: 7_920_000_000_000_000},
		},
	}

	artifact, encoded, err := BuildArtifact(cfg)
	if err != nil {
		t.Fatalf("BuildArtifact() error = %v", err)
	}
	if artifact.Commitment == "" {
		t.Fatal("BuildArtifact() returned an empty commitment")
	}

	path := filepath.Join(t.TempDir(), "production-artifact.json")
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := tokenomics.Load(path); err == nil {
		t.Fatal("node GenesisState loader unexpectedly accepted production Artifact JSON")
	}
}
