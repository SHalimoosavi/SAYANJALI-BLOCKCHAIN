package block

import "testing"

func TestV2MerkleVector(t *testing.T) {
	got := MerkleRoot([]string{
		"4cf093632005df1a52730685d2a251e42d2b0a35e3f8179c4966f9a0f04d81c4",
		"f2d1b0239244c5c1b70805b15eba42a9e99ca32c764f17e267398136159136ac",
	})
	want := "e6d735b0462b043ad8af37a3424121c06b85fc1f70ceef8de5db7fc9819085e6"
	if got != want {
		t.Fatalf("V2 merkle root = %s want %s", got, want)
	}
}
