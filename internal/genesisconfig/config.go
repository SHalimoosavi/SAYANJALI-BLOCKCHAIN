package genesisconfig

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/canonicaljson"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

type Environment string

const (
	PrivateTestnet Environment = "private-testnet"
	PublicTestnet  Environment = "public-testnet"
	Production     Environment = "production"
	Mainnet        Environment = "mainnet"
)

type Allocation struct {
	Category        string `json:"category"`
	Address         string `json:"address"`
	AmountBaseUnits uint64 `json:"amount_base_units"`
}

type Config struct {
	NetworkName         string       `json:"network_name"`
	Environment         Environment  `json:"environment"`
	GenesisStateVersion string       `json:"genesis_state_version"`
	TokenomicsVersion   string       `json:"tokenomics_version"`
	Allocations         []Allocation `json:"allocations"`
}

type Artifact struct {
	NetworkName           string       `json:"network_name"`
	Environment           Environment  `json:"environment"`
	GenesisStateVersion   string       `json:"genesis_state_version"`
	TokenomicsVersion     string       `json:"tokenomics_version"`
	Allocations           []Allocation `json:"allocations"`
	GenesisTotalBaseUnits uint64       `json:"genesis_total_base_units"`
	MiningBaseUnits       uint64       `json:"mining_base_units"`
	MaxSupplyBaseUnits    uint64       `json:"max_supply_base_units"`
	Commitment            string       `json:"commitment"`
}

func Validate(c Config) error {
	if c.NetworkName == "" {
		return errors.New("network_name is required")
	}
	if c.GenesisStateVersion != "1" {
		return errors.New("genesis_state_version must be 1")
	}
	if c.TokenomicsVersion != "1" {
		return errors.New("tokenomics_version must be 1")
	}

	switch c.Environment {
	case PrivateTestnet, PublicTestnet, Production, Mainnet:
	default:
		return fmt.Errorf("invalid environment %q", c.Environment)
	}

	if len(c.Allocations) != 5 {
		return fmt.Errorf("exactly five allocations are required, got %d", len(c.Allocations))
	}

	seenCategory := make(map[string]bool, 5)
	seenAddress := make(map[string]bool, 5)

	expected := map[string]uint64{
		string(tokenomics.Presale):   7_200_000_000_000_000,
		string(tokenomics.Treasury):  5_760_000_000_000_000,
		string(tokenomics.Ecosystem): 3_600_000_000_000_000,
		string(tokenomics.Liquidity): 4_320_000_000_000_000,
		string(tokenomics.Team):      7_920_000_000_000_000,
	}

	var total uint64
	for _, a := range c.Allocations {
		if seenCategory[a.Category] {
			return fmt.Errorf("duplicate allocation category %q", a.Category)
		}
		seenCategory[a.Category] = true

		if _, ok := expected[a.Category]; !ok {
			return fmt.Errorf("unknown allocation category %q", a.Category)
		}
		if a.AmountBaseUnits != expected[a.Category] {
			return fmt.Errorf("wrong amount for %s: got %d want %d", a.Category, a.AmountBaseUnits, expected[a.Category])
		}
		if a.Address == "" {
			return fmt.Errorf("empty address for %s", a.Category)
		}
		if !wallet.ValidAddress(a.Address) {
			return fmt.Errorf("invalid SYJ address for %s", a.Category)
		}
		if seenAddress[a.Address] {
			return fmt.Errorf("duplicate recipient address %q", a.Address)
		}
		seenAddress[a.Address] = true

		if total > ^uint64(0)-a.AmountBaseUnits {
			return errors.New("allocation total overflows uint64")
		}
		total += a.AmountBaseUnits
	}

	if total != tokenomics.ExpectedGenesisTotal() {
		return fmt.Errorf("genesis total mismatch: got %d want %d", total, tokenomics.ExpectedGenesisTotal())
	}

	if c.Environment == Production || c.Environment == Mainnet {
		if strings.Contains(strings.ToLower(c.NetworkName), "mvp") {
			return errors.New("production/mainnet configuration cannot use an MVP network identity")
		}
		if c.NetworkName != tokenomics.Phase7NetworkName {
			return fmt.Errorf("production/mainnet network_name must be %q", tokenomics.Phase7NetworkName)
		}
	}

	return nil
}

func ToGenesisState(c Config) (tokenomics.GenesisState, error) {
	if err := Validate(c); err != nil {
		return tokenomics.GenesisState{}, err
	}

	allocations := make([]tokenomics.Allocation, 0, len(c.Allocations))
	for _, a := range c.Allocations {
		allocations = append(allocations, tokenomics.Allocation{
			Category:        tokenomics.Category(a.Category),
			Recipient:       a.Address,
			AmountBaseUnits: a.AmountBaseUnits,
		})
	}

	gs := tokenomics.GenesisState{
		Version:        tokenomics.Version,
		Sender:         protocol.GenesisAllocationSender,
		Allocations:    allocations,
		TotalBaseUnits: tokenomics.ExpectedGenesisTotal(),
	}
	if err := gs.Validate(wallet.ValidAddress); err != nil {
		return tokenomics.GenesisState{}, err
	}
	return gs, nil
}

func CanonicalBytes(c Config) ([]byte, error) {
	gs, err := ToGenesisState(c)
	if err != nil {
		return nil, err
	}
	stateBytes, err := gs.CanonicalBytes()
	if err != nil {
		return nil, err
	}

	var state any
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		return nil, fmt.Errorf("invalid canonical genesis state: %w", err)
	}

	payload := map[string]any{
		"environment":           string(c.Environment),
		"genesis_state_version": c.GenesisStateVersion,
		"network_name":          c.NetworkName,
		"state":                 state,
		"tokenomics_version":    c.TokenomicsVersion,
	}
	return canonicaljson.Marshal(payload)
}

func Commitment(c Config) (string, []byte, error) {
	b, err := CanonicalBytes(c)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:]), b, nil
}

func BuildArtifact(c Config) (Artifact, []byte, error) {
	commitment, _, err := Commitment(c)
	if err != nil {
		return Artifact{}, nil, err
	}

	artifact := Artifact{
		NetworkName:           c.NetworkName,
		Environment:           c.Environment,
		GenesisStateVersion:   c.GenesisStateVersion,
		TokenomicsVersion:     c.TokenomicsVersion,
		Allocations:           append([]Allocation(nil), c.Allocations...),
		GenesisTotalBaseUnits: tokenomics.ExpectedGenesisTotal(),
		MiningBaseUnits:       tokenomics.MiningAllocationBaseUnits(),
		MaxSupplyBaseUnits:    protocol.MaxSupplyBaseUnits,
		Commitment:            commitment,
	}

	allocations := make([]any, 0, len(artifact.Allocations))
	for _, a := range artifact.Allocations {
		allocations = append(allocations, map[string]any{
			"address":           a.Address,
			"amount_base_units": a.AmountBaseUnits,
			"category":          a.Category,
		})
	}

	b, err := canonicaljson.Marshal(map[string]any{
		"allocations":              allocations,
		"commitment":               artifact.Commitment,
		"environment":              string(artifact.Environment),
		"genesis_state_version":    artifact.GenesisStateVersion,
		"genesis_total_base_units": artifact.GenesisTotalBaseUnits,
		"max_supply_base_units":    artifact.MaxSupplyBaseUnits,
		"mining_base_units":        artifact.MiningBaseUnits,
		"network_name":             artifact.NetworkName,
		"tokenomics_version":       artifact.TokenomicsVersion,
	})
	if err != nil {
		return Artifact{}, nil, err
	}
	return artifact, b, nil
}

func VerifyArtifact(a Artifact) error {
	if a.Commitment == "" {
		return errors.New("artifact commitment is required")
	}
	c := Config{
		NetworkName:         a.NetworkName,
		Environment:         a.Environment,
		GenesisStateVersion: a.GenesisStateVersion,
		TokenomicsVersion:   a.TokenomicsVersion,
		Allocations:         a.Allocations,
	}
	commitment, _, err := Commitment(c)
	if err != nil {
		return err
	}
	if commitment != a.Commitment {
		return fmt.Errorf("artifact commitment mismatch: got %s want %s", a.Commitment, commitment)
	}
	if a.GenesisTotalBaseUnits != tokenomics.ExpectedGenesisTotal() {
		return errors.New("artifact genesis total mismatch")
	}
	if a.MiningBaseUnits != tokenomics.MiningAllocationBaseUnits() {
		return errors.New("artifact mining allocation mismatch")
	}
	if a.MaxSupplyBaseUnits != protocol.MaxSupplyBaseUnits {
		return errors.New("artifact maximum supply mismatch")
	}
	return nil
}

func LoadConfig(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("config path is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if containsSensitiveFields(b) {
		return Config{}, errors.New("sensitive/private key material is not permitted in genesis configuration")
	}

	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("invalid genesis configuration: %w", err)
	}
	if err := Validate(c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func LoadArtifact(path string) (Artifact, error) {
	if path == "" {
		return Artifact{}, errors.New("artifact path is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, err
	}
	if containsSensitiveFields(b) {
		return Artifact{}, errors.New("sensitive/private key material is not permitted in genesis artifact")
	}

	var a Artifact
	if err := json.Unmarshal(b, &a); err != nil {
		return Artifact{}, fmt.Errorf("invalid genesis artifact: %w", err)
	}
	if err := VerifyArtifact(a); err != nil {
		return Artifact{}, err
	}
	return a, nil
}

func containsSensitiveFields(b []byte) bool {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return false
	}
	return scanSensitive(v)
}

func scanSensitive(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, value := range x {
			switch strings.ToLower(k) {
			case "private_key", "privatekey", "seed", "seed_phrase", "mnemonic",
				"password", "secret", "credential", "credentials", "token",
				"api_key", "apikey":
				return true
			}
			if scanSensitive(value) {
				return true
			}
		}
	case []any:
		for _, value := range x {
			if scanSensitive(value) {
				return true
			}
		}
	}
	return false
}
