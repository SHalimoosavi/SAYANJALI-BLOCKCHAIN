package networkid

import "testing"

func TestEffectiveNetworkIDAuditedValue(t *testing.T) {
	got, _, err := EffectiveNetworkID(Inputs{
		ConsensusProtocolVersion: 2,
		GenesisStateCommitment:   "36351980711cd88fe6ff134f0e4e858ee1a4572a9f44b7bde9f57213e7f1eb82",
		HistoricalGenesisHash:    "5c95d2d7b63bde94fbfcfe4c2a95850ca96e0fc649e93e545b8b32f3a9c7cc1b",
		NetworkName:              "sayanjali-syj-phase7-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3"
	if got != want {
		t.Fatalf("network id = %s want %s", got, want)
	}
}

func TestNetworkIDRejectsDesignTimeAlternative(t *testing.T) {
	if ValidateHex64("539059f933550ccd507fbd8d8791df0671a1e77ea54e7c2f857c808c47660e72") != nil {
		t.Fatal("alternative value is syntactically invalid")
	}
	got, _, err := EffectiveNetworkID(Inputs{
		ConsensusProtocolVersion: 2,
		GenesisStateCommitment:   "36351980711cd88fe6ff134f0e4e858ee1a4572a9f44b7bde9f57213e7f1eb82",
		HistoricalGenesisHash:    "5c95d2d7b63bde94fbfcfe4c2a95850ca96e0fc649e93e545b8b32f3a9c7cc1b",
		NetworkName:              "sayanjali-syj-phase7-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got == "539059f933550ccd507fbd8d8791df0671a1e77ea54e7c2f857c808c47660e72" {
		t.Fatal("design-time alternative was silently accepted")
	}
}

func TestWireNetworkName(t *testing.T) {
	id := "237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3"
	got, err := WireNetworkName(id)
	if err != nil {
		t.Fatal(err)
	}
	want := "syjnet-v2-" + id
	if got != want {
		t.Fatalf("wire network name = %s want %s", got, want)
	}
}

func TestV2WireNameFitsFrozenP2PStringLimit(t *testing.T) {
	id := "237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3"
	wire, err := WireNetworkName(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) > 256 {
		t.Fatalf("wire network name is %d bytes", len(wire))
	}
}
