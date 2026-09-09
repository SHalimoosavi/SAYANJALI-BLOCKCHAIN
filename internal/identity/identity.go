package identity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
)

const (
	encryptionKeyEnv = "SYJ_IDENTITY_ENCRYPTION_KEY"
	allowLegacyEnv   = "SYJ_ALLOW_LEGACY_PLAINTEXT_IDENTITY"
	identityFilePerm = 0600
	identityDirPerm  = 0700
)

type Identity struct {
	NodeID              string `json:"node_id"`
	PrivateKeyHex       string `json:"-"`
	EncryptedPrivateKey string `json:"private_key_encrypted,omitempty"`
	LegacyPrivateKeyHex string `json:"private_key_hex,omitempty"`
	PublicKeyHex        string `json:"public_key_hex"`
	Address             string `json:"address"`
}

func LoadOrCreate(dir string) (*Identity, bool, error) {
	if err := os.MkdirAll(dir, identityDirPerm); err != nil {
		return nil, false, err
	}
	if err := os.Chmod(dir, identityDirPerm); err != nil {
		return nil, false, fmt.Errorf("protect identity directory: %w", err)
	}
	path := filepath.Join(dir, "identity.json")
	data, err := os.ReadFile(path)
	if err == nil {
		if err := requirePrivateFile(path); err != nil {
			return nil, false, err
		}
		var stored Identity
		if err := json.Unmarshal(data, &stored); err != nil {
			return nil, false, fmt.Errorf("decode identity: %w", err)
		}
		if stored.EncryptedPrivateKey != "" {
			key, err := encryptionKey()
			if err != nil {
				return nil, false, err
			}
			stored.PrivateKeyHex, err = decryptPrivateKey(stored.EncryptedPrivateKey, key)
			if err != nil {
				return nil, false, err
			}
		} else if legacyPlaintextAllowed() && stored.LegacyPrivateKeyHex != "" {
			stored.PrivateKeyHex = stored.LegacyPrivateKeyHex
			stored.LegacyPrivateKeyHex = ""
		} else if !legacyPlaintextAllowed() {
			return nil, false, errors.New("unencrypted identity rejected; configure SYJ_IDENTITY_ENCRYPTION_KEY or explicitly enable legacy plaintext compatibility")
		}
		if err := stored.Validate(); err != nil {
			return nil, false, err
		}
		return &stored, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}

	key, err := encryptionKey()
	if err != nil {
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
	id.EncryptedPrivateKey, err = encryptPrivateKey(id.PrivateKeyHex, key)
	if err != nil {
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
	if err = tmp.Chmod(identityFilePerm); err == nil {
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
	if err = os.Chmod(path, identityFilePerm); err != nil {
		return nil, false, err
	}
	return id, true, nil
}

func encryptionKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(encryptionKeyEnv))
	if raw == "" {
		return nil, fmt.Errorf("%s is required for encrypted node identity storage", encryptionKeyEnv)
	}
	key, err := hex.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s must be exactly 32 bytes encoded as 64 hex characters", encryptionKeyEnv)
	}
	return key, nil
}

func legacyPlaintextAllowed() bool { return os.Getenv(allowLegacyEnv) == "1" }

func requirePrivateFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("identity file %s has insecure permissions %04o; require owner-only access", path, info.Mode().Perm())
	}
	return nil
}

func encryptPrivateKey(privateKeyHex string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(privateKeyHex), nil)
	return hex.EncodeToString(append(nonce, ciphertext...)), nil
}

func decryptPrivateKey(encoded string, key []byte) (string, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		return "", errors.New("invalid encrypted identity encoding")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize()+gcm.Overhead() {
		return "", errors.New("encrypted identity is truncated")
	}
	plaintext, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("encrypted identity authentication failed")
	}
	return string(plaintext), nil
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
func mustHex(s string) []byte          { b, _ := hex.DecodeString(s); return b }
func RandomChallenge() ([]byte, error) { b := make([]byte, 32); _, err := rand.Read(b); return b, err }
