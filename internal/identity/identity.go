package identity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
)

type Identity struct {
	NodeID        string `json:"node_id"`
	PrivateKeyHex string `json:"private_key_hex"`
	PublicKeyHex  string `json:"public_key_hex"`
	Address       string `json:"address"`
}

func LoadOrCreate(dir string) (*Identity, bool, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, false, err
	}
	path := filepath.Join(dir, "identity.json")
	data, err := os.ReadFile(path)
	if err == nil {
		var id Identity
		if err := json.Unmarshal(data, &id); err != nil {
			return nil, false, fmt.Errorf("decode identity: %w", err)
		}
		if err := id.Validate(); err != nil {
			return nil, false, err
		}
		return &id, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	kp, err := wallet.New()
	if err != nil {
		return nil, false, err
	}
	nodeIDBytes := corecrypto.SHA256Bytes([]byte(kp.PublicKeyHex))
	id := &Identity{NodeID: hex.EncodeToString(nodeIDBytes[:]), PrivateKeyHex: kp.PrivateKeyHex, PublicKeyHex: kp.PublicKeyHex, Address: kp.Address}
	if err := id.Validate(); err != nil {
		return nil, false, err
	}
	encoded, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return nil, false, err
	}
	tmp, err := os.CreateTemp(dir, "identity-*.tmp")
	if err != nil {
		return nil, false, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(encoded)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, false, err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return nil, false, err
	}
	return id, true, nil
}

func (id Identity) Validate() error {
	if len(id.PrivateKeyHex) != 64 || strings.Trim(id.PrivateKeyHex, "0123456789abcdefABCDEF") != "" {
		return errors.New("invalid identity private key")
	}
	if len(id.PublicKeyHex) != 128 {
		return errors.New("invalid identity public key")
	}
	kp, err := wallet.FromPrivateKeyHex(id.PrivateKeyHex)
	if err != nil {
		return fmt.Errorf("invalid identity key: %w", err)
	}
	if kp.PublicKeyHex != id.PublicKeyHex || kp.Address != id.Address {
		return errors.New("identity public key/address mismatch")
	}
	expected := corecrypto.SHA256Bytes([]byte(id.PublicKeyHex))
	if id.NodeID != hex.EncodeToString(expected[:]) {
		return errors.New("identity node_id mismatch")
	}
	return nil
}

func (id Identity) Sign(message []byte) ([]byte, error) {
	return corecrypto.SignECDSA(mustHex(id.PrivateKeyHex), string(message))
}
func mustHex(s string) []byte { b, _ := hex.DecodeString(s); return b }

// RandomChallenge returns the exact 32-byte challenge required by the frozen handshake.
func RandomChallenge() ([]byte, error) { b := make([]byte, 32); _, err := rand.Read(b); return b, err }
