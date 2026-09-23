package networkid

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/canonicaljson"
)

const (
	EffectiveAlgorithmDomain = "SYJ-EFFECTIVE-NETWORK-ID-V1\x00"
	V2WirePrefix             = "syjnet-v2-"
)

type Inputs struct {
	ConsensusProtocolVersion uint64 `json:"consensus_protocol_version"`
	GenesisStateCommitment   string `json:"genesis_state_commitment"`
	HistoricalGenesisHash    string `json:"historical_genesis_hash"`
	NetworkName              string `json:"network_name"`
}

func ValidateHex64(s string) error {
	if len(s) != 64 {
		return errors.New("network id must be exactly 64 hexadecimal characters")
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return errors.New("network id must contain lowercase hexadecimal characters only")
		}
	}
	return nil
}

func EffectiveNetworkID(in Inputs) (string, []byte, error) {
	if in.ConsensusProtocolVersion != 2 {
		return "", nil, errors.New("effective network id requires consensus protocol version 2")
	}
	if err := ValidateHex64(in.GenesisStateCommitment); err != nil {
		return "", nil, fmt.Errorf("invalid genesis state commitment: %w", err)
	}
	if err := ValidateHex64(in.HistoricalGenesisHash); err != nil {
		return "", nil, fmt.Errorf("invalid historical genesis hash: %w", err)
	}
	if in.NetworkName == "" {
		return "", nil, errors.New("network name is required")
	}
	payload := map[string]any{
		"consensus_protocol_version": in.ConsensusProtocolVersion,
		"genesis_state_commitment":   in.GenesisStateCommitment,
		"historical_genesis_hash":    in.HistoricalGenesisHash,
		"network_name":               in.NetworkName,
	}
	canonical, err := canonicaljson.Marshal(payload)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(append([]byte(EffectiveAlgorithmDomain), canonical...))
	return hex.EncodeToString(sum[:]), canonical, nil
}

func WireNetworkName(id string) (string, error) {
	if err := ValidateHex64(id); err != nil {
		return "", err
	}
	return V2WirePrefix + id, nil
}
