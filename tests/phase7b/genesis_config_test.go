package phase7b_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/genesisconfig"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

func testAddress(seed byte) string {
	var pub [64]byte
	for i := range pub {
		pub[i] = seed
	}
	return wallet.AddressFromPublicKeyHex(stringHex(pub[:]))
}

func stringHex(b []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&15]
	}
	return string(out)
}

func validConfig() genesisconfig.Config {
	a := []string{
		testAddress(1),
		testAddress(2),
		testAddress(3),
		testAddress(4),
		testAddress(5),
	}

	return genesisconfig.Config{
		NetworkName:         tokenomics.Phase7NetworkName,
		Environment:         genesisconfig.PublicTestnet,
		GenesisStateVersion: "1",
		TokenomicsVersion:   "1",
		Allocations: []genesisconfig.Allocation{
			{Category: "presale", Address: a[0], AmountBaseUnits: 7_200_000_000_000_000},
			{Category: "treasury", Address: a[1], AmountBaseUnits: 5_760_000_000_000_000},
			{Category: "ecosystem", Address: a[2], AmountBaseUnits: 3_600_000_000_000_000},
			{Category: "liquidity", Address: a[3], AmountBaseUnits: 4_320_000_000_000_000},
			{Category: "team", Address: a[4], AmountBaseUnits: 7_920_000_000_000_000},
		},
	}
}

func TestExactEconomics(t *testing.T) {
	c := validConfig()

	if err := genesisconfig.Validate(c); err != nil {
		t.Fatal(err)
	}

	gs, err := genesisconfig.ToGenesisState(c)
	if err != nil {
		t.Fatal(err)
	}

	if gs.TotalBaseUnits != tokenomics.ExpectedGenesisTotal() {
		t.Fatalf("genesis total = %d", gs.TotalBaseUnits)
	}
	if tokenomics.MiningAllocationBaseUnits()+gs.TotalBaseUnits != protocol.MaxSupplyBaseUnits {
		t.Fatal("supply plan mismatch")
	}
}

func TestOrderIndependentCommitment(t *testing.T) {
	c := validConfig()

	h1, _, err := genesisconfig.Commitment(c)
	if err != nil {
		t.Fatal(err)
	}

	c.Allocations[0], c.Allocations[4] = c.Allocations[4], c.Allocations[0]

	h2, _, err := genesisconfig.Commitment(c)
	if err != nil {
		t.Fatal(err)
	}

	if h1 != h2 {
		t.Fatalf("allocation order changed commitment: %s != %s", h1, h2)
	}
}

func TestDuplicateAddressRejected(t *testing.T) {
	c := validConfig()
	c.Allocations[1].Address = c.Allocations[0].Address

	if err := genesisconfig.Validate(c); err == nil {
		t.Fatal("duplicate address accepted")
	}
}

func TestWrongAmountRejected(t *testing.T) {
	c := validConfig()
	c.Allocations[0].AmountBaseUnits++

	if err := genesisconfig.Validate(c); err == nil {
		t.Fatal("wrong allocation accepted")
	}
}

func TestInvalidAddressRejected(t *testing.T) {
	c := validConfig()
	c.Allocations[0].Address = "not-an-address"

	if err := genesisconfig.Validate(c); err == nil {
		t.Fatal("invalid address accepted")
	}
}

func TestExtraAllocationRejected(t *testing.T) {
	c := validConfig()
	c.Allocations = append(c.Allocations, genesisconfig.Allocation{
		Category:        "extra",
		Address:         testAddress(6),
		AmountBaseUnits: 1,
	})

	if err := genesisconfig.Validate(c); err == nil {
		t.Fatal("extra allocation accepted")
	}
}

func TestProductionNetworkFailsClosed(t *testing.T) {
	c := validConfig()
	c.Environment = genesisconfig.Production
	c.NetworkName = "sayanjali-mainnet-mvp"

	if err := genesisconfig.Validate(c); err == nil {
		t.Fatal("MVP production network accepted")
	}
}

func TestProductionEmptyAddressRejected(t *testing.T) {
	c := validConfig()
	c.Environment = genesisconfig.Production
	c.Allocations[0].Address = ""

	if err := genesisconfig.Validate(c); err == nil {
		t.Fatal("empty production address accepted")
	}
}

func TestSensitiveConfigFieldRejected(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "bad.json")

	b, err := json.Marshal(map[string]any{
		"network_name":          tokenomics.Phase7NetworkName,
		"environment":           "public-testnet",
		"genesis_state_version": "1",
		"tokenomics_version":    "1",
		"private_key":           "deadbeef",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := genesisconfig.LoadConfig(p); err == nil {
		t.Fatal("sensitive field accepted")
	}
}

func TestArtifactRoundTripVerification(t *testing.T) {
	c := validConfig()

	art, _, err := genesisconfig.BuildArtifact(c)
	if err != nil {
		t.Fatal(err)
	}

	if err := genesisconfig.VerifyArtifact(art); err != nil {
		t.Fatal(err)
	}
}

func TestMissingCommitmentFailsVerification(t *testing.T) {
	c := validConfig()

	art, _, err := genesisconfig.BuildArtifact(c)
	if err != nil {
		t.Fatal(err)
	}

	art.Commitment = ""

	if err := genesisconfig.VerifyArtifact(art); err == nil {
		t.Fatal("missing commitment verified")
	}
}

func TestArtifactTamperFailsVerification(t *testing.T) {
	c := validConfig()

	art, _, err := genesisconfig.BuildArtifact(c)
	if err != nil {
		t.Fatal(err)
	}

	art.Allocations[0].AmountBaseUnits++

	if err := genesisconfig.VerifyArtifact(art); err == nil {
		t.Fatal("tampered artifact verified")
	}
}
