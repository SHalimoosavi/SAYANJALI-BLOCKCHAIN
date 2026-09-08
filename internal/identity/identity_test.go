package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreatePersistsIdentity(t *testing.T) {
	dir := t.TempDir()
	first, created, err := LoadOrCreate(dir)
	if err != nil || !created {
		t.Fatalf("first load: created=%v err=%v", created, err)
	}
	second, created, err := LoadOrCreate(dir)
	if err != nil || created {
		t.Fatalf("restart load: created=%v err=%v", created, err)
	}
	if first.NodeID != second.NodeID || first.PrivateKeyHex != second.PrivateKeyHex || first.PublicKeyHex != second.PublicKeyHex || first.Address != second.Address {
		t.Fatal("identity changed across restart")
	}
}
func TestCorruptIdentityRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := LoadOrCreate(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "identity.json"), []byte(`{"node_id":"bad"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadOrCreate(dir); err == nil {
		t.Fatal("expected corrupt identity error")
	}
}
