package chain

import (
	"errors"
	"fmt"
	"math"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/consensus"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/state"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

func validateChainV2(ch []*block.Block, genesisState *tokenomics.GenesisState, networkID string) error {
	if genesisState == nil {
		return errors.New("V2 chain requires GenesisState")
	}
	if len(ch) == 0 {
		return errors.New("empty chain")
	}
	if err := block.ValidateGenesis(ch[0]); err != nil {
		return err
	}
	cfg := protocol.DefaultDifficultyConfig()
	for i := 1; i < len(ch); i++ {
		if err := validateNextV2(ch[i], ch[:i], cfg, genesisState, networkID); err != nil {
			return fmt.Errorf("block %d: %w", ch[i].Index, err)
		}
	}
	return nil
}

func validateNextV2(b *block.Block, prefix []*block.Block, cfg protocol.DifficultyConfig, genesisState *tokenomics.GenesisState, networkID string) error {
	if genesisState == nil {
		return errors.New("V2 chain requires GenesisState")
	}
	if len(prefix) == 0 {
		return errors.New("V2 block requires a parent")
	}
	p := prefix[len(prefix)-1]
	if b.Index != p.Index+1 {
		return errors.New("non-sequential block index")
	}
	if b.PreviousHash != p.Hash {
		return errors.New("previous hash mismatch")
	}
	if err := validateTimestamp(b.Timestamp, prefix); err != nil {
		return err
	}
	mined, err := miningIssuedFor(prefix)
	if err != nil {
		return err
	}
	reward, ok := consensus.ExpectedMiningReward(mined)
	if !ok {
		return errors.New("no issuance remains")
	}
	if err := validateV2TransactionsAndState(prefix, b, genesisState, networkID, reward); err != nil {
		return err
	}
	expected, err := requiredNextDifficulty(prefix, cfg)
	if err != nil {
		return err
	}
	if b.Difficulty != expected {
		return fmt.Errorf("difficulty %d != required %d", b.Difficulty, expected)
	}
	if err := consensus.ValidatePoW(b, expected); err != nil {
		return err
	}
	return nil
}

func initialV2State(genesisState *tokenomics.GenesisState) (state.Balances, map[string]uint64, uint64, error) {
	if genesisState == nil {
		return nil, nil, 0, errors.New("V2 GenesisState is required")
	}
	if err := genesisState.Validate(wallet.ValidAddress); err != nil {
		return nil, nil, 0, err
	}
	balances := make(state.Balances)
	nonces := make(map[string]uint64)
	var genesisSupply uint64
	for _, a := range genesisState.Allocations {
		if err := balances.Credit(a.Recipient, a.AmountBaseUnits); err != nil {
			return nil, nil, 0, err
		}
		if ^uint64(0)-genesisSupply < a.AmountBaseUnits {
			return nil, nil, 0, errors.New("genesis supply overflow")
		}
		genesisSupply += a.AmountBaseUnits
	}
	if genesisSupply != tokenomics.ExpectedGenesisTotal() {
		return nil, nil, 0, errors.New("genesis supply mismatch")
	}
	return balances, nonces, genesisSupply, nil
}

func cloneBalances(in state.Balances) state.Balances {
	out := make(state.Balances, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneNonces(in map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func v2CoinbaseValid(tx transaction.Transaction, network string, reward uint64) error {
	if !tx.IsV2() {
		return errors.New("V2 block contains non-V2 coinbase")
	}
	if tx.Sender != protocol.CoinbaseSender {
		return errors.New("invalid V2 coinbase sender")
	}
	if tx.NetworkID != network {
		return errors.New("V2 coinbase network mismatch")
	}
	if tx.SenderPublicKey != "" || tx.Signature != "" {
		return errors.New("V2 coinbase must be unsigned")
	}
	if tx.Nonce != 0 {
		return errors.New("V2 coinbase nonce must be zero")
	}
	if tx.AmountBaseUnits != reward {
		return errors.New("V2 coinbase reward mismatch")
	}
	if !wallet.ValidAddress(tx.Receiver) {
		return errors.New("invalid V2 coinbase receiver")
	}
	if math.IsNaN(tx.Timestamp) || math.IsInf(tx.Timestamp, 0) || tx.Timestamp < 0 {
		return errors.New("invalid V2 coinbase timestamp")
	}
	want, err := tx.ComputeV2TxID()
	if err != nil {
		return err
	}
	if tx.TxID != want {
		return errors.New("V2 coinbase tx_id mismatch")
	}
	return nil
}

func validateV2TransactionsAndState(prefix []*block.Block, b *block.Block, genesisState *tokenomics.GenesisState, networkID string, reward uint64) error {
	balances, nonces, _, err := replayV2State(prefix, genesisState, networkID)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{})
	for _, pb := range prefix {
		for _, tx := range pb.Transactions {
			id := tx.IdentityHash()
			if id == "" {
				return errors.New("V2 transaction identity is empty")
			}
			seen[id] = struct{}{}
		}
	}
	coinbases := 0
	blockSeen := make(map[string]struct{}, len(b.Transactions))
	for _, tx := range b.Transactions {
		id := tx.IdentityHash()
		if id == "" {
			return errors.New("V2 transaction identity is empty")
		}
		if _, ok := seen[id]; ok {
			return errors.New("duplicate transaction identity already confirmed")
		}
		if _, ok := blockSeen[id]; ok {
			return errors.New("duplicate transaction identity in block")
		}
		blockSeen[id] = struct{}{}
		if tx.Sender == protocol.CoinbaseSender {
			coinbases++
			if err := v2CoinbaseValid(tx, networkID, reward); err != nil {
				return err
			}
			if err := balances.Credit(tx.Receiver, tx.AmountBaseUnits); err != nil {
				return err
			}
			continue
		}
		if err := tx.ValidateV2(networkID); err != nil {
			return err
		}
		expected := nonces[tx.Sender]
		if tx.Nonce != expected {
			return fmt.Errorf("invalid nonce for %s: got %d want %d", tx.Sender, tx.Nonce, expected)
		}
		if expected == ^uint64(0) {
			return errors.New("sender nonce exhausted at uint64 maximum")
		}
		if err := balances.Debit(tx.Sender, tx.AmountBaseUnits); err != nil {
			return err
		}
		if err := balances.Credit(tx.Receiver, tx.AmountBaseUnits); err != nil {
			return err
		}
		nonces[tx.Sender] = expected + 1
	}
	if coinbases != 1 {
		return errors.New("exactly one V2 coinbase required")
	}
	hashes := make([]string, len(b.Transactions))
	for i := range b.Transactions {
		hashes[i] = b.Transactions[i].IdentityHash()
	}
	if block.MerkleRoot(hashes) != b.MerkleRoot {
		return errors.New("V2 merkle root mismatch")
	}
	h, _, err := block.HashHeader(b.Header)
	if err != nil {
		return err
	}
	if h != b.Hash {
		return errors.New("block hash mismatch")
	}
	return nil
}

func replayV2State(ch []*block.Block, genesisState *tokenomics.GenesisState, networkID string) (state.Balances, map[string]uint64, uint64, error) {
	balances, nonces, genesisSupply, err := initialV2State(genesisState)
	if err != nil {
		return nil, nil, 0, err
	}
	confirmed := make(map[string]struct{})
	var miningIssued uint64
	var supply = genesisSupply
	for i := 1; i < len(ch); i++ {
		b := ch[i]
		mined, err := miningIssuedFor(ch[:i])
		if err != nil {
			return nil, nil, 0, err
		}
		reward, ok := consensus.ExpectedMiningReward(mined)
		if !ok {
			return nil, nil, 0, errors.New("no issuance remains")
		}
		blockSeen := make(map[string]struct{}, len(b.Transactions))
		coinbases := 0
		for _, tx := range b.Transactions {
			id := tx.IdentityHash()
			if id == "" {
				return nil, nil, 0, errors.New("V2 transaction identity is empty")
			}
			if _, ok := confirmed[id]; ok {
				return nil, nil, 0, errors.New("duplicate V2 transaction during replay")
			}
			if _, ok := blockSeen[id]; ok {
				return nil, nil, 0, errors.New("duplicate V2 transaction in block replay")
			}
			blockSeen[id] = struct{}{}
			if tx.Sender == protocol.CoinbaseSender {
				coinbases++
				if err := v2CoinbaseValid(tx, networkID, reward); err != nil {
					return nil, nil, 0, err
				}
				if err := balances.Credit(tx.Receiver, tx.AmountBaseUnits); err != nil {
					return nil, nil, 0, err
				}
				if ^uint64(0)-miningIssued < tx.AmountBaseUnits || ^uint64(0)-supply < tx.AmountBaseUnits {
					return nil, nil, 0, errors.New("issuance overflow")
				}
				miningIssued += tx.AmountBaseUnits
				supply += tx.AmountBaseUnits
			} else {
				if err := tx.ValidateV2(networkID); err != nil {
					return nil, nil, 0, err
				}
				expected := nonces[tx.Sender]
				if tx.Nonce != expected {
					return nil, nil, 0, fmt.Errorf("invalid nonce during replay: got %d want %d", tx.Nonce, expected)
				}
				if expected == ^uint64(0) {
					return nil, nil, 0, errors.New("sender nonce exhausted at uint64 maximum")
				}
				if err := balances.Debit(tx.Sender, tx.AmountBaseUnits); err != nil {
					return nil, nil, 0, err
				}
				if err := balances.Credit(tx.Receiver, tx.AmountBaseUnits); err != nil {
					return nil, nil, 0, err
				}
				nonces[tx.Sender] = expected + 1
			}
			confirmed[id] = struct{}{}
		}
		if coinbases != 1 {
			return nil, nil, 0, errors.New("exactly one V2 coinbase required during replay")
		}
		hashes := make([]string, len(b.Transactions))
		for j := range b.Transactions {
			hashes[j] = b.Transactions[j].IdentityHash()
		}
		if block.MerkleRoot(hashes) != b.MerkleRoot {
			return nil, nil, 0, errors.New("V2 merkle mismatch during replay")
		}
	}
	if supply > protocol.MaxSupplyBaseUnits {
		return nil, nil, 0, errors.New("maximum supply exceeded")
	}
	return balances, nonces, miningIssued, nil
}

func (c *Chain) replayStateV2(ch []*block.Block) error {
	balances, nonces, miningIssued, err := replayV2State(ch, c.genesisState, c.networkID)
	if err != nil {
		return err
	}
	var genesisSupply uint64
	for _, a := range c.genesisState.Allocations {
		genesisSupply += a.AmountBaseUnits
	}
	c.balances = balances
	c.nonces = nonces
	c.genesisSupply = genesisSupply
	c.miningIssued = miningIssued
	c.supply = genesisSupply + miningIssued
	c.confirmedTx = make(map[string]struct{})
	for i := 1; i < len(ch); i++ {
		for _, tx := range ch[i].Transactions {
			c.confirmedTx[tx.IdentityHash()] = struct{}{}
		}
	}
	return nil
}
