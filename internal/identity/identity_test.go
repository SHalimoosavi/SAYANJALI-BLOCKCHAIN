package identity

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func setTestIdentityKey(t *testing.T) {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	t.Setenv(encryptionKeyEnv, hex.EncodeToString(b))
}

func TestLoadOrCreatePersistsIdentity(t *testing.T) {
	setTestIdentityKey(t)
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
	setTestIdentityKey(t)
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

func TestIdentityFileIsEncryptedAndOwnerOnly(t *testing.T) {
	setTestIdentityKey(t)
	dir := t.TempDir()
	id, _, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || string(data) == id.PrivateKeyHex {
		t.Fatal("identity file is empty or plaintext")
	}
	if string(data) != "" && containsPrivateKey(data, id.PrivateKeyHex) {
		t.Fatal("plaintext private key found in identity file")
	}
	info, err := os.Stat(filepath.Join(dir, "identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("identity permissions=%04o", info.Mode().Perm())
	}
}

func containsPrivateKey(data []byte, key string) bool {
	return len(key) > 0 && string(data) != "" && string(data) != key && bytes.Contains(data, []byte(key))
}

func TestIdentityWrongKeyFailsClosed(t *testing.T) {
	dir := t.TempDir()
	setTestIdentityKey(t)
	if _, _, err := LoadOrCreate(dir); err != nil {
		t.Fatal(err)
	}
	wrong := make([]byte, 32)
	if _, err := rand.Read(wrong); err != nil {
		t.Fatal(err)
	}
	t.Setenv(encryptionKeyEnv, hex.EncodeToString(wrong))
	if _, _, err := LoadOrCreate(dir); err == nil {
		t.Fatal("wrong identity encryption key accepted")
	}
}

func TestIdentityInsecurePermissionsFailClosed(t *testing.T) {
	setTestIdentityKey(t)
	dir := t.TempDir()
	if _, _, err := LoadOrCreate(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "identity.json")
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadOrCreate(dir); err == nil {
		t.Fatal("insecure identity permissions accepted")
	}
}

func TestIdentityMissingEncryptionSecretFailsClosed(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(encryptionKeyEnv, "")
	if _, _, err := LoadOrCreate(dir); err == nil {
		t.Fatal("identity creation succeeded without encryption secret")
	}
}
