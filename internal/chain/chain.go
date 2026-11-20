package chain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/clock"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/consensus"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/networkid"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/state"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/statecommitment"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

type chainStore interface {
	AllBlocks() ([]*block.Block, error)
	SaveBlock(*block.Block) error
	SetTip(string) error
	Tip() string
	SaveStateCheckpoint(string, []byte) error
	GetStateCheckpoint(string) ([]byte, bool)
	Close() error
}

type Chain struct {
	mu              sync.RWMutex
	store           chainStore
	blocks          map[string]*block.Block
	activeTip       string
	active          []*block.Block
	balances        state.Balances
	supply          uint64
	genesisSupply   uint64
	miningIssued    uint64
	genesisState    *tokenomics.GenesisState
	confirmedTx     map[string]struct{}
	protocolVersion uint8
	networkID       string
	networkName     string
	clock           clock.Clock
	nonces          map[string]uint64
	stateSnapshots  map[string]*statecommitment.Snapshot
	pendingPromotions map[string]struct{}
	stateRoot       string
	reorgCandidates []transaction.Transaction
}

func Open(store *storage.Store) (*Chain, error) {
	return openWithProtocol(store, nil, 1, "", "")
}

// OpenV3WithGenesisState opens the V3 C-1 protocol while preserving the
// existing V2 economic/state architecture. V3 Merkle activation is explicitly
// not enabled by this constructor.
func OpenV3WithGenesisState(store *storage.Store, genesisState tokenomics.GenesisState, networkID, networkName string) (*Chain, error) {
	if err := genesisState.Validate(wallet.ValidAddress); err != nil {
		return nil, err
	}
	if err := networkid.ValidateHex64(networkID); err != nil {
		return nil, err
	}
	wireName, err := networkid.WireNetworkName(networkID)
	if err != nil {
		return nil, err
	}
	if networkName != wireName {
		return nil, errors.New("V3 network name does not match network id")
	}
	return openWithProtocol(store, &genesisState, 3, networkID, networkName)
}

// OpenV2WithGenesisState opens a V2 consensus chain bound to an explicit network identity.
// The historical block genesis remains frozen; V2 is separated by transaction rules and network identity.
func OpenV2WithGenesisState(store *storage.Store, genesisState tokenomics.GenesisState, networkID, networkName string) (*Chain, error) {
	if err := genesisState.Validate(wallet.ValidAddress); err != nil {
		return nil, err
	}
	if err := networkid.ValidateHex64(networkID); err != nil {
		return nil, err
	}
	wireName, err := networkid.WireNetworkName(networkID)
	if err != nil {
		return nil, err
	}
	if networkName != wireName {
		return nil, errors.New("V2 network name does not match network id")
	}
	return openWithProtocol(store, &genesisState, 2, networkID, networkName)
}

// OpenWithGenesisState opens a Phase 7 economic chain while preserving the
// historical frozen block genesis. The economic genesis is deterministic
// initial state, not a block or transaction.
func OpenWithGenesisState(store *storage.Store, genesisState tokenomics.GenesisState) (*Chain, error) {
	if err := genesisState.Validate(wallet.ValidAddress); err != nil {
		return nil, err
	}
	return openWithProtocol(store, &genesisState, 1, "", tokenomics.Phase7NetworkName)
}

func openWithProtocol(store *storage.Store, genesisState *tokenomics.GenesisState, protocolVersion uint8, networkID, networkName string) (*Chain, error) {
	return openWithProtocolAndClock(store, genesisState, protocolVersion, networkID, networkName, clock.RealClock{})
}

func openWithProtocolAndClock(store chainStore, genesisState *tokenomics.GenesisState, protocolVersion uint8, networkID, networkName string, clk clock.Clock) (*Chain, error) {
	if clk == nil {
		clk = clock.RealClock{}
	}
	c := &Chain{
		store:           store,
		blocks:          make(map[string]*block.Block),
		balances:        make(state.Balances),
		confirmedTx:     make(map[string]struct{}),
		genesisState:    genesisState,
		protocolVersion: protocolVersion,
		networkID:       networkID,
		networkName:     networkName,
		clock:           clk,
		nonces:          make(map[string]uint64),
		stateSnapshots:  make(map[string]*statecommitment.Snapshot),
		pendingPromotions: make(map[string]struct{}),
	}
	bs, err := store.AllBlocks()
	if err != nil {
		return nil, err
	}
	for _, b := range bs {
		c.blocks[b.Hash] = b
	}
	g, err := block.Genesis()
	if err != nil {
		return nil, err
	}
	if len(bs) == 0 {
		if err := store.SaveBlock(g); err != nil {
			return nil, err
		}
		if err := store.SetTip(g.Hash); err != nil {
			return nil, err
		}
		c.blocks[g.Hash] = g
	}
	genesisBlock, ok := c.blocks[g.Hash]
	if !ok {
		return nil, errors.New("stored database does not contain the frozen genesis block")
	}
	if err := block.ValidateGenesis(genesisBlock); err != nil {
		return nil, err
	}
	tip := store.Tip()
	if tip == "" {
		tip = g.Hash
	}
	if _, ok := c.blocks[tip]; !ok {
		return nil, errors.New("stored best tip is missing")
	}
	candidate, err := c.buildChain(tip)
	if err != nil {
		return nil, err
	}
	if err := c.validateChain(candidate); err != nil {
		return nil, err
	}

	// The durable tip is the tie-break anchor. Reconcile only to strictly
	// greater-work stored chains, preserving the existing equal-work rule.
	bestTip := tip
	bestChain := candidate
	bestWork := work(candidate)
	hasChild := make(map[string]struct{}, len(bs))
	for _, stored := range bs {
		if stored.Index > 0 {
			hasChild[stored.PreviousHash] = struct{}{}
		}
	}
	for _, stored := range bs {
		if _, hasStoredChild := hasChild[stored.Hash]; hasStoredChild {
			continue
		}
		storedChain, buildErr := c.buildChain(stored.Hash)
		if buildErr != nil {
			continue
		}
		if validateErr := c.validateChain(storedChain); validateErr != nil {
			// A malformed leaf must not hide a valid ancestor.
			validPrefix := false
			for end := len(storedChain) - 2; end >= 0; end-- {
				prefix := storedChain[:end+1]
				if validateChainErr := c.validateChain(prefix); validateChainErr == nil {
					storedChain = prefix
					validPrefix = true
					break
				}
			}
			if !validPrefix {
				continue
			}
		}
		storedWork := work(storedChain)
		if storedWork.Cmp(bestWork) > 0 {
			bestWork = storedWork
			bestChain = storedChain
			bestTip = storedChain[len(storedChain)-1].Hash
		}
	}
	c.activeTip = bestTip
	c.active = bestChain
	if err := c.replayState(bestChain); err != nil {
		return nil, err
	}
	if err := c.initializeIncrementalState(bestChain); err != nil {
		return nil, err
	}
	if bestTip != tip {
		if err := store.SetTip(bestTip); err != nil {
			return nil, fmt.Errorf("repair durable best tip: %w", err)
		}
	}
	return c, nil
}
func ValidateChain(ch []*block.Block) error {
	return validateChainWithState(ch, nil)
}

func (c *Chain) validateChain(ch []*block.Block) error {
	if c.protocolVersion >= 2 {
		return validateChainV2(ch, c.genesisState, c.networkID, c.protocolVersion)
	}
	return validateChainWithState(ch, c.genesisState)
}

func validateChainWithState(ch []*block.Block, genesisState *tokenomics.GenesisState) error {
	if len(ch) == 0 {
		return errors.New("empty chain")
	}
	if err := block.ValidateGenesis(ch[0]); err != nil {
		return err
	}
	cfg := protocol.DefaultDifficultyConfig()
	for i := 1; i < len(ch); i++ {
		if err := validateNext(ch[i], ch[:i], cfg, genesisState, 1); err != nil {
			return fmt.Errorf("block %d: %w", ch[i].Index, err)
		}
	}
	return nil
}

func validateNext(b *block.Block, prefix []*block.Block, cfg protocol.DifficultyConfig, genesisState *tokenomics.GenesisState, protocolVersion uint8) error {
	p := prefix[len(prefix)-1]
	if b.Index != p.Index+1 {
		return errors.New("non-sequential block index")
	}
	if b.PreviousHash != p.Hash {
		return errors.New("previous hash mismatch")
	}
	if err := validateTimestampForProtocol(b.Timestamp, prefix, protocolVersion); err != nil {
		return err
	}
	var reward uint64
	var ok bool
	if genesisState != nil {
		mined, err := miningIssuedFor(prefix)
		if err != nil {
			return err
		}
		reward, ok = consensus.ExpectedMiningReward(mined)
	} else {
		reward, ok = consensus.ExpectedReward(supplyFor(prefix))
	}
	if !ok {
		return errors.New("no issuance remains")
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
	if err := transactionsValid(prefix, b, reward); err != nil {
		return err
	}
	if err := stateTransition(prefix, b, reward, genesisState); err != nil {
		return err
	}
	return nil
}

// requiredNextDifficulty applies the frozen retarget algorithm only at a
// complete 10-block epoch boundary. Before then, the previous mineable
// difficulty remains in force. This keeps the Go node from feeding a short
// window into the frozen Python Fraction-based retarget routine.
const (
	// TimestampFutureStepSeconds is the frozen V1/V2 parent-relative bound.
	// V3 does not use this as its future-time ceiling.
	TimestampFutureStepSeconds = int64(4) * protocol.TargetBlockTimeSeconds
	TimestampMTPWindow         = 11
)

func medianTimePast(prefix []*block.Block) (float64, error) {
	if len(prefix) == 0 {
		return 0, errors.New("median-time-past requires a parent")
	}
	start := len(prefix) - TimestampMTPWindow
	if start < 0 {
		start = 0
	}
	vals := make([]float64, 0, len(prefix)-start)
	for i := start; i < len(prefix); i++ {
		vals = append(vals, prefix[i].Timestamp)
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	return vals[len(vals)/2], nil
}

func validateTimestampForProtocol(timestamp float64, prefix []*block.Block, protocolVersion uint8) error {
	if len(prefix) == 0 {
		return errors.New("timestamp validation requires a parent")
	}
	if timestamp != timestamp || timestamp < 0 || timestamp > float64(^uint64(0)) || timestamp != float64(int64(timestamp)) {
		return errors.New("invalid block timestamp: consensus timestamps must be integer seconds")
	}
	parent := prefix[len(prefix)-1].Timestamp
	mtp, err := medianTimePast(prefix)
	if err != nil {
		return err
	}
	if timestamp <= mtp {
		return errors.New("timestamp must be greater than median-time-past")
	}
	if timestamp <= parent {
		return errors.New("timestamp must increase")
	}
	if protocolVersion < 3 {
		max := parent + float64(TimestampFutureStepSeconds)
		if timestamp > max {
			return fmt.Errorf("timestamp exceeds deterministic future-time bound %.0f", max)
		}
	}
	return nil
}

func requiredNextDifficulty(prefix []*block.Block, cfg protocol.DifficultyConfig) (int, error) {
	if len(prefix) == 0 {
		return 4, nil
	}
	base := 4
	if prefix[len(prefix)-1].Index > 0 {
		base = prefix[len(prefix)-1].Difficulty
	}
	nextIndex := int64(len(prefix))
	if len(prefix) < protocol.DifficultyInterval || nextIndex%protocol.DifficultyInterval != 0 {
		return base, nil
	}
	recent := prefix[len(prefix)-protocol.DifficultyInterval:]
	return consensus.NextDifficulty(recent, cfg, 4)
}

func supplyFor(ch []*block.Block) uint64 {
	var s uint64
	for _, b := range ch {
		if b.Index == 0 {
			continue
		}
		for _, tx := range b.Transactions {
			if tx.Sender == protocol.CoinbaseSender {
				s += tx.AmountBaseUnits
			}
		}
	}
	return s
}
func stateTransition(prefix []*block.Block, b *block.Block, reward uint64, genesisState *tokenomics.GenesisState) error {
	balances, genesisSupply, miningIssued, supply, err := replayBalances(prefix, genesisState)
	if err != nil {
		return err
	}
	if genesisState != nil {
		if genesisSupply != tokenomics.ExpectedGenesisTotal() {
			return errors.New("invalid Phase 7 genesis supply")
		}
		if _, err := tokenomics.NewSupplyPlan(genesisSupply); err != nil {
			return err
		}
	}
	coinbases := 0
	for _, tx := range b.Transactions {
		if tx.Sender == protocol.GenesisAllocationSender {
			return errors.New("genesis allocation cannot be a transaction")
		}
		if tx.Sender == protocol.CoinbaseSender {
			coinbases++
			if tx.AmountBaseUnits != reward {
				return errors.New("invalid coinbase reward")
			}
			if !balancesCredit(balances, tx.Receiver, tx.AmountBaseUnits) {
				return errors.New("balance overflow")
			}
			if ^uint64(0)-miningIssued < tx.AmountBaseUnits {
				return errors.New("mining issuance overflow")
			}
			miningIssued += tx.AmountBaseUnits
			if ^uint64(0)-supply < tx.AmountBaseUnits {
				return errors.New("supply overflow")
			}
			supply += tx.AmountBaseUnits
		} else {
			if err := tx.Validate(); err != nil {
				return err
			}
			if err := balances.Debit(tx.Sender, tx.AmountBaseUnits); err != nil {
				return err
			}
			if err := balances.Credit(tx.Receiver, tx.AmountBaseUnits); err != nil {
				return err
			}
		}
	}
	if coinbases != 1 {
		return errors.New("exactly one coinbase required")
	}
	if genesisState != nil {
		plan, err := tokenomics.NewSupplyPlan(genesisSupply)
		if err != nil {
			return err
		}
		if err := plan.ValidateMiningIssued(miningIssued); err != nil {
			return err
		}
	}
	if supply > protocol.MaxSupplyBaseUnits {
		return errors.New("maximum supply exceeded")
	}
	return nil
}

func balancesCredit(b state.Balances, a string, v uint64) bool { return b.Credit(a, v) == nil }

func replayBalances(ch []*block.Block, genesisState *tokenomics.GenesisState) (state.Balances, uint64, uint64, uint64, error) {
	balances := make(state.Balances)
	var genesisSupply uint64
	if genesisState != nil {
		if err := genesisState.Validate(wallet.ValidAddress); err != nil {
			return nil, 0, 0, 0, err
		}
		for _, a := range genesisState.Allocations {
			if !balancesCredit(balances, a.Recipient, a.AmountBaseUnits) {
				return nil, 0, 0, 0, errors.New("genesis balance overflow")
			}
			if ^uint64(0)-genesisSupply < a.AmountBaseUnits {
				return nil, 0, 0, 0, errors.New("genesis supply overflow")
			}
			genesisSupply += a.AmountBaseUnits
		}
		if genesisSupply != tokenomics.ExpectedGenesisTotal() {
			return nil, 0, 0, 0, errors.New("genesis supply mismatch")
		}
	}
	var miningIssued uint64
	var supply = genesisSupply
	seenTx := make(map[string]struct{})
	for _, b := range ch {
		if b.Index == 0 {
			continue
		}
		for _, tx := range b.Transactions {
			id := tx.IdentityHash()
			if id != "" {
				if _, exists := seenTx[id]; exists {
					return nil, 0, 0, 0, errors.New("duplicate transaction identity in active chain")
				}
				seenTx[id] = struct{}{}
			}
			if tx.Sender == protocol.GenesisAllocationSender {
				return nil, 0, 0, 0, errors.New("genesis allocation cannot appear in a block")
			}
			if tx.Sender == protocol.CoinbaseSender {
				if !balancesCredit(balances, tx.Receiver, tx.AmountBaseUnits) {
					return nil, 0, 0, 0, errors.New("balance overflow")
				}
				if ^uint64(0)-miningIssued < tx.AmountBaseUnits || ^uint64(0)-supply < tx.AmountBaseUnits {
					return nil, 0, 0, 0, errors.New("issuance overflow")
				}
				miningIssued += tx.AmountBaseUnits
				supply += tx.AmountBaseUnits
			} else {
				if err := tx.Validate(); err != nil {
					return nil, 0, 0, 0, err
				}
				if err := balances.Debit(tx.Sender, tx.AmountBaseUnits); err != nil {
					return nil, 0, 0, 0, err
				}
				if err := balances.Credit(tx.Receiver, tx.AmountBaseUnits); err != nil {
					return nil, 0, 0, 0, err
				}
			}
		}
	}
	if genesisState != nil {
		plan, err := tokenomics.NewSupplyPlan(genesisSupply)
		if err != nil {
			return nil, 0, 0, 0, err
		}
		if err := plan.ValidateMiningIssued(miningIssued); err != nil {
			return nil, 0, 0, 0, err
		}
	} else if supply > protocol.MaxSupplyBaseUnits {
		return nil, 0, 0, 0, errors.New("maximum supply exceeded")
	}
	return balances, genesisSupply, miningIssued, supply, nil
}

func miningIssuedFor(ch []*block.Block) (uint64, error) {
	var total uint64
	for _, b := range ch {
		if b.Index == 0 {
			continue
		}
		for _, tx := range b.Transactions {
			if tx.Sender == protocol.GenesisAllocationSender {
				return 0, errors.New("genesis allocation cannot appear in a block")
			}
			if tx.Sender == protocol.CoinbaseSender {
				if ^uint64(0)-total < tx.AmountBaseUnits {
					return 0, errors.New("mining issuance overflow")
				}
				total += tx.AmountBaseUnits
			}
		}
	}
	return total, nil
}

// transactionsValid performs structural transaction/coinbase checks without mutating chain state.
func transactionsValid(prefix []*block.Block, b *block.Block, reward uint64) error {
	if b.Index == 0 {
		return block.ValidateGenesis(b)
	}
	coin := 0
	seen := make(map[string]struct{}, len(b.Transactions))
	for _, tx := range b.Transactions {
		if err := tx.Validate(); err != nil {
			return fmt.Errorf("invalid transaction: %w", err)
		}
		id := tx.IdentityHash()
		if id == "" {
			return errors.New("transaction identity is empty")
		}
		if _, exists := seen[id]; exists {
			return errors.New("duplicate transaction identity within block")
		}
		seen[id] = struct{}{}
		for _, prior := range prefix {
			for _, confirmed := range prior.Transactions {
				if confirmed.IdentityHash() == id {
					return errors.New("transaction identity already confirmed")
				}
			}
		}
		if tx.Sender == protocol.CoinbaseSender {
			coin++
			if tx.SenderPublicKey != "" || tx.Signature != "" {
				return errors.New("coinbase must be unsigned")
			}
			if tx.AmountBaseUnits != reward {
				return errors.New("coinbase reward mismatch")
			}
		}
	}
	if coin != 1 {
		return errors.New("exactly one coinbase required")
	}
	hashes := make([]string, len(b.Transactions))
	for i := range b.Transactions {
		hashes[i] = b.Transactions[i].TxHash
	}
	if block.MerkleRoot(hashes) != b.MerkleRoot {
		return errors.New("merkle root mismatch")
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

func (c *Chain) buildChain(tip string) ([]*block.Block, error) {
	rev := make([]*block.Block, 0)
	cur, ok := c.blocks[tip]
	if !ok {
		return nil, errors.New("unknown tip")
	}
	for {
		rev = append(rev, cur)
		if cur.Index == 0 {
			break
		}
		p, ok := c.blocks[cur.PreviousHash]
		if !ok {
			return nil, fmt.Errorf("missing ancestor %s", cur.PreviousHash)
		}
		cur = p
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, nil
}
func (c *Chain) replayState(ch []*block.Block) error {
	if c.protocolVersion >= 2 {
		return c.replayStateV2(ch)
	}
	balances, genesisSupply, miningIssued, supply, err := replayBalances(ch, c.genesisState)
	if err != nil {
		return err
	}
	c.balances = balances
	c.genesisSupply = genesisSupply
	c.miningIssued = miningIssued
	c.supply = supply
	c.confirmedTx = make(map[string]struct{})
	for _, b := range ch {
		if b.Index == 0 {
			continue
		}
		for _, tx := range b.Transactions {
			if tx.TxHash != "" {
				c.confirmedTx[tx.TxHash] = struct{}{}
			}
		}
	}
	return nil
}

const stateCheckpointInterval int64 = 16

func genesisSnapshot(genesisState *tokenomics.GenesisState) (*statecommitment.Snapshot, error) {
	s := statecommitment.NewSnapshot()
	if genesisState == nil {
		return s, nil
	}
	if err := genesisState.Validate(wallet.ValidAddress); err != nil {
		return nil, err
	}
	for _, a := range genesisState.Allocations {
		if err := sCredit(s, a.Recipient, a.AmountBaseUnits); err != nil {
			return nil, err
		}
		if ^uint64(0)-s.GenesisSupply < a.AmountBaseUnits {
			return nil, errors.New("genesis supply overflow")
		}
		s.GenesisSupply += a.AmountBaseUnits
	}
	if s.GenesisSupply != tokenomics.ExpectedGenesisTotal() {
		return nil, errors.New("genesis supply mismatch")
	}
	s.Supply = s.GenesisSupply
	return s, nil
}

func sCredit(s *statecommitment.Snapshot, addr string, amount uint64) error {
	cur := s.Balances[addr]
	if ^uint64(0)-cur < amount {
		return errors.New("balance overflow")
	}
	s.Balances[addr] = cur + amount
	return nil
}
func sDebit(s *statecommitment.Snapshot, addr string, amount uint64) error {
	cur := s.Balances[addr]
	if amount > cur {
		return errors.New("insufficient balance")
	}
	s.Balances[addr] = cur - amount
	return nil
}

func applyIncrementalV1(s *statecommitment.Snapshot, b *block.Block, reward uint64, genesisState *tokenomics.GenesisState) error {
	coin := 0
	for _, tx := range b.Transactions {
		if err := tx.Validate(); err != nil {
			return err
		}
		if tx.Sender == protocol.GenesisAllocationSender {
			return errors.New("genesis allocation cannot appear in a block")
		}
		if tx.Sender == protocol.CoinbaseSender {
			coin++
			if tx.SenderPublicKey != "" || tx.Signature != "" {
				return errors.New("coinbase must be unsigned")
			}
			if tx.AmountBaseUnits != reward {
				return errors.New("invalid coinbase reward")
			}
			if err := sCredit(s, tx.Receiver, tx.AmountBaseUnits); err != nil {
				return err
			}
			if ^uint64(0)-s.MiningIssued < tx.AmountBaseUnits || ^uint64(0)-s.Supply < tx.AmountBaseUnits {
				return errors.New("issuance overflow")
			}
			s.MiningIssued += tx.AmountBaseUnits
			s.Supply += tx.AmountBaseUnits
		} else {
			if err := sDebit(s, tx.Sender, tx.AmountBaseUnits); err != nil {
				return err
			}
			if err := sCredit(s, tx.Receiver, tx.AmountBaseUnits); err != nil {
				return err
			}
		}
	}
	if coin != 1 {
		return errors.New("exactly one coinbase required")
	}
	if genesisState != nil {
		plan, err := tokenomics.NewSupplyPlan(s.GenesisSupply)
		if err != nil {
			return err
		}
		if err := plan.ValidateMiningIssued(s.MiningIssued); err != nil {
			return err
		}
	} else if s.Supply > protocol.MaxSupplyBaseUnits {
		return errors.New("maximum supply exceeded")
	}
	return nil
}

func applyIncrementalV2(s *statecommitment.Snapshot, b *block.Block, reward uint64, networkID string) error {
	coin := 0
	for _, tx := range b.Transactions {
		id := tx.IdentityHash()
		if id == "" {
			return errors.New("V2 transaction identity is empty")
		}
		if tx.Sender == protocol.CoinbaseSender {
			coin++
			if err := v2CoinbaseValid(tx, networkID, reward); err != nil {
				return err
			}
			if err := sCredit(s, tx.Receiver, tx.AmountBaseUnits); err != nil {
				return err
			}
			if ^uint64(0)-s.MiningIssued < tx.AmountBaseUnits || ^uint64(0)-s.Supply < tx.AmountBaseUnits {
				return errors.New("issuance overflow")
			}
			s.MiningIssued += tx.AmountBaseUnits
			s.Supply += tx.AmountBaseUnits
			continue
		}
		if err := tx.ValidateV2(networkID); err != nil {
			return err
		}
		expected := s.Nonces[tx.Sender]
		if tx.Nonce != expected {
			return fmt.Errorf("invalid nonce for %s: got %d want %d", tx.Sender, tx.Nonce, expected)
		}
		if expected == ^uint64(0) {
			return errors.New("sender nonce exhausted at uint64 maximum")
		}
		if err := sDebit(s, tx.Sender, tx.AmountBaseUnits); err != nil {
			return err
		}
		if err := sCredit(s, tx.Receiver, tx.AmountBaseUnits); err != nil {
			return err
		}
		s.Nonces[tx.Sender] = expected + 1
	}
	if coin != 1 {
		return errors.New("exactly one V2 coinbase required")
	}
	if s.Supply > protocol.MaxSupplyBaseUnits {
		return errors.New("maximum supply exceeded")
	}
	return nil
}

func applyIncrementalBlock(s *statecommitment.Snapshot, b *block.Block, reward uint64, genesisState *tokenomics.GenesisState, protocolVersion uint8, networkID string) error {
	if b.Index == 0 {
		return nil
	}
	if protocolVersion >= 2 {
		return applyIncrementalV2(s, b, reward, networkID)
	}
	return applyIncrementalV1(s, b, reward, genesisState)
}

func validateV2TransactionsAndStateStructure(prefix []*block.Block, b *block.Block, networkID string, reward uint64) error {
	seen := make(map[string]struct{}, len(prefix)+len(b.Transactions))
	for _, pb := range prefix {
		for _, tx := range pb.Transactions {
			id := tx.IdentityHash()
			if id == "" {
				return errors.New("V2 transaction identity is empty")
			}
			seen[id] = struct{}{}
		}
	}
	blockSeen := make(map[string]struct{}, len(b.Transactions))
	coin := 0
	for _, tx := range b.Transactions {
		id := tx.IdentityHash()
		if id == "" {
			return errors.New("V2 transaction identity is empty")
		}
		if _, ok := seen[id]; ok {
			return errors.New("duplicate V2 transaction identity already confirmed")
		}
		if _, ok := blockSeen[id]; ok {
			return errors.New("duplicate V2 transaction identity in block")
		}
		blockSeen[id] = struct{}{}
		if tx.Sender == protocol.CoinbaseSender {
			coin++
			if err := v2CoinbaseValid(tx, networkID, reward); err != nil {
				return err
			}
		} else if err := tx.ValidateV2(networkID); err != nil {
			return err
		}
	}
	if coin != 1 {
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

func validateIncrementalBlock(b *block.Block, prefix []*block.Block, parent *statecommitment.Snapshot, cfg protocol.DifficultyConfig, genesisState *tokenomics.GenesisState, protocolVersion uint8, networkID string) (*statecommitment.Snapshot, error) {
	if len(prefix) == 0 {
		return nil, errors.New("empty chain prefix")
	}
	p := prefix[len(prefix)-1]
	if b.Index != p.Index+1 {
		return nil, errors.New("non-sequential block index")
	}
	if b.PreviousHash != p.Hash {
		return nil, errors.New("previous hash mismatch")
	}
	if err := validateTimestampForProtocol(b.Timestamp, prefix, protocolVersion); err != nil {
		return nil, err
	}
	var reward uint64
	var ok bool
	if protocolVersion >= 2 {
		reward, ok = consensus.ExpectedMiningReward(parent.MiningIssued)
	} else if genesisState != nil {
		reward, ok = consensus.ExpectedMiningReward(parent.MiningIssued)
	} else {
		reward, ok = consensus.ExpectedReward(parent.Supply)
	}
	if !ok {
		return nil, errors.New("no issuance remains")
	}
	expected, err := requiredNextDifficulty(prefix, cfg)
	if err != nil {
		return nil, err
	}
	if b.Difficulty != expected {
		return nil, fmt.Errorf("difficulty %d != required %d", b.Difficulty, expected)
	}
	if protocolVersion >= 2 {
		if err := validateV2TransactionsAndStateStructure(prefix, b, networkID, reward); err != nil {
			return nil, err
		}
	} else if err := transactionsValid(prefix, b, reward); err != nil {
		return nil, err
	}
	if err := consensus.ValidatePoW(b, expected); err != nil {
		return nil, err
	}
	candidate := parent.Clone()
	if err := applyIncrementalBlock(candidate, b, reward, genesisState, protocolVersion, networkID); err != nil {
		return nil, err
	}
	return candidate, nil
}

func (c *Chain) initializeIncrementalState(ch []*block.Block) error {
	if len(ch) == 0 {
		return errors.New("cannot initialize state from empty chain")
	}
	base, err := genesisSnapshot(c.genesisState)
	if err != nil {
		return err
	}
	start := 1
	for i := len(ch) - 1; i >= 0; i-- {
		if raw, ok := c.store.GetStateCheckpoint(ch[i].Hash); ok {
			decoded, err := statecommitment.Decode(raw)
			if err != nil {
				return fmt.Errorf("state checkpoint decode: %w", err)
			}
			base = decoded
			start = i + 1
			break
		}
	}
	c.stateSnapshots = make(map[string]*statecommitment.Snapshot)
	if start == 1 {
		c.stateSnapshots[ch[0].Hash] = base.Clone()
	}
	current := base
	for i := start; i < len(ch); i++ {
		b := ch[i]
		next, err := validateIncrementalBlock(b, ch[:i], current, protocol.DefaultDifficultyConfig(), c.genesisState, c.protocolVersion, c.networkID)
		if err != nil {
			return fmt.Errorf("incremental state at block %d: %w", b.Index, err)
		}
		current = next
		if b.Index%stateCheckpointInterval == 0 {
			if err := c.store.SaveStateCheckpoint(b.Hash, statecommitment.Encode(current)); err != nil {
				return err
			}
		}
	}
	c.stateSnapshots[ch[len(ch)-1].Hash] = current.Clone()
	c.stateRoot = current.Root()
	return nil
}

func (c *Chain) stateForParent(prefix []*block.Block) (*statecommitment.Snapshot, error) {
	parent := prefix[len(prefix)-1]
	if s, ok := c.stateSnapshots[parent.Hash]; ok {
		return s.Clone(), nil
	}
	// Forks are reconstructed only from the nearest known state checkpoint/common ancestor,
	// never by replaying the complete active history.
	base, err := genesisSnapshot(c.genesisState)
	if err != nil {
		return nil, err
	}
	start := 1
	for i := len(prefix) - 1; i >= 0; i-- {
		if raw, ok := c.store.GetStateCheckpoint(prefix[i].Hash); ok {
			decoded, err := statecommitment.Decode(raw)
			if err != nil {
				return nil, fmt.Errorf("state checkpoint decode: %w", err)
			}
			base = decoded
			start = i + 1
			break
		}
	}
	for i := start; i < len(prefix); i++ {
		b := prefix[i]
		next, err := validateIncrementalBlock(b, prefix[:i], base, protocol.DefaultDifficultyConfig(), c.genesisState, c.protocolVersion, c.networkID)
		if err != nil {
			return nil, err
		}
		base = next
		c.stateSnapshots[b.Hash] = base.Clone()
	}
	return base.Clone(), nil
}

func work(ch []*block.Block) *big.Int { return consensus.ChainWork(ch) }
func (c *Chain) Accept(b *block.Block) (bool, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, stored := c.blocks[b.Hash]; stored {
		active := false
		for _, activeBlock := range c.active {
			if activeBlock.Hash == b.Hash {
				active = true
				break
			}
		}
		if active {
			return false, "duplicate", nil
		}
		if _, pending := c.pendingPromotions[b.Hash]; !pending {
			return false, "duplicate", nil
		}
	}
	if b.Index == 0 {
		return false, "genesis", errors.New("genesis already exists")
	}

	prefix, err := c.buildChain(b.PreviousHash)
	if err != nil {
		return false, "unknown_ancestor", err
	}

	if c.protocolVersion >= 3 {
		if err := c.validateV3IngressTimestamp(b.Timestamp, prefix); err != nil {
			return false, "invalid_ingress_time", err
		}
	}

	parentState, err := c.stateForParent(prefix)
	if err != nil {
		return false, "state", err
	}

	if _, err := validateIncrementalBlock(
		b,
		prefix,
		parentState,
		protocol.DefaultDifficultyConfig(),
		c.genesisState,
		c.protocolVersion,
		c.networkID,
	); err != nil {
		return false, "invalid", err
	}

	candidate := append(append([]*block.Block(nil), prefix...), b)

	// Persist a new block before making it eligible to become the durable tip.
	if _, stored := c.blocks[b.Hash]; !stored {
		if err := c.store.SaveBlock(b); err != nil {
			return false, "storage", err
		}
		c.blocks[b.Hash] = b
	}

	if work(candidate).Cmp(work(c.active)) > 0 {
		// Remember a persisted candidate whose promotion is still pending.
		// This permits retry after checkpoint or tip persistence failures.
		c.pendingPromotions[b.Hash] = struct{}{}

		// Derive the complete winning state before changing either the
		// durable tip or the in-memory active chain.
		winningState, err := c.stateForParent(prefix)
		if err != nil {
			return false, "state", err
		}

		nextState, err := validateIncrementalBlock(
			b,
			prefix,
			winningState,
			protocol.DefaultDifficultyConfig(),
			c.genesisState,
			c.protocolVersion,
			c.networkID,
		)
		if err != nil {
			return false, "state", err
		}

		// Prepare the checkpoint before advancing the durable tip. If
		// checkpoint persistence fails, the previous durable tip remains
		// authoritative and the active in-memory chain is unchanged.
		if b.Index%stateCheckpointInterval == 0 {
			payload := statecommitment.Encode(nextState)
			if err := c.store.SaveStateCheckpoint(b.Hash, payload); err != nil {
				return false, "storage", err
			}
		}

		// Only after all state derivation and required persistence have
		// succeeded do we advance the durable tip.
		if err := c.store.SetTip(b.Hash); err != nil {
			return false, "storage", err
		}
		delete(c.pendingPromotions, b.Hash)

		if c.protocolVersion >= 2 {
			common := -1
			limit := len(prefix)
			if len(c.active) < limit {
				limit = len(c.active)
			}
			for i := 0; i < limit; i++ {
				if c.active[i].Hash == prefix[i].Hash {
					common = i
				} else {
					break
				}
			}

			c.reorgCandidates = c.reorgCandidates[:0]
			if common >= 0 {
				for i := common + 1; i < len(c.active); i++ {
					for _, tx := range c.active[i].Transactions {
						if tx.Sender != protocol.CoinbaseSender && tx.IsV2() {
							c.reorgCandidates = append(c.reorgCandidates, tx)
						}
					}
				}
			}
		}

		// Durable persistence has succeeded; now publish the new active
		// state in memory.
		c.activeTip = b.Hash
		c.active = candidate

		c.stateSnapshots[b.Hash] = nextState.Clone()
		c.stateRoot = nextState.Root()

		c.balances = make(state.Balances)
		for a, v := range nextState.Balances {
			c.balances[a] = v
		}

		c.nonces = make(map[string]uint64)
		for a, v := range nextState.Nonces {
			c.nonces[a] = v
		}

		c.genesisSupply = nextState.GenesisSupply
		c.miningIssued = nextState.MiningIssued
		c.supply = nextState.Supply

		c.confirmedTx = make(map[string]struct{})
		for _, ab := range candidate {
			if ab.Index == 0 {
				continue
			}
			for _, tx := range ab.Transactions {
				c.confirmedTx[tx.IdentityHash()] = struct{}{}
			}
		}

		return true, "best", nil
	}

	delete(c.pendingPromotions, b.Hash)
	return true, "fork", nil
}

func (c *Chain) SetClock(clk clock.Clock) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if clk == nil {
		clk = clock.RealClock{}
	}
	c.clock = clk
}

func (c *Chain) MedianTimePast() (time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.active) == 0 {
		return time.Time{}, false
	}
	mtp, err := medianTimePast(c.active)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(int64(mtp), 0), true
}

func (c *Chain) validateV3IngressTimestamp(timestamp float64, prefix []*block.Block) error {
	if err := validateTimestampForProtocol(timestamp, prefix, 3); err != nil {
		return err
	}
	now := c.clock.Now().Unix()
	max := float64(now + int64(protocol.V3FutureTimeBound/time.Second))
	if timestamp > max {
		return fmt.Errorf("V3 ingress timestamp exceeds current clock by more than %d seconds", int64(protocol.V3FutureTimeBound/time.Second))
	}
	return nil
}

func (c *Chain) Tip() *block.Block {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active[len(c.active)-1]
}
func (c *Chain) TipHash() string { c.mu.RLock(); defer c.mu.RUnlock(); return c.activeTip }
func (c *Chain) Height() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active[len(c.active)-1].Index
}
func (c *Chain) Work() *big.Int { c.mu.RLock(); defer c.mu.RUnlock(); return work(c.active) }
func (c *Chain) ChainCopy() []*block.Block {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]*block.Block(nil), c.active...)
}
func (c *Chain) Balance(addr string) uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.balances[addr]
}
func (c *Chain) Supply() uint64 { c.mu.RLock(); defer c.mu.RUnlock(); return c.supply }
func (c *Chain) HasBlock(hash string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.blocks[hash]
	return ok
}
func (c *Chain) GetBlockByHeight(height int64) (*block.Block, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if height < 0 || height >= int64(len(c.active)) {
		return nil, errors.New("height not found")
	}
	b := c.active[height]
	if b.Index != height {
		return nil, errors.New("active chain height/index mismatch")
	}
	return b, nil
}

func (c *Chain) GetBlock(hash string) (*block.Block, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	b, ok := c.blocks[hash]
	if !ok {
		return nil, errors.New("not found")
	}
	return b, nil
}

// HasConfirmedTransaction reports whether txHash is present in the active chain.
// The index intentionally excludes transactions that exist only on losing forks.
func (c *Chain) HasConfirmedTransaction(txHash string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.confirmedTx[txHash]
	return ok
}

// TakeReorgCandidates returns V2 transactions from the losing active branch.
// The caller must revalidate them against the rebuilt winning-chain state before mempool admission.
func (c *Chain) TakeReorgCandidates() []transaction.Transaction {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := append([]transaction.Transaction(nil), c.reorgCandidates...)
	c.reorgCandidates = nil
	return out
}

func (c *Chain) GenesisState() (tokenomics.GenesisState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.genesisState == nil {
		return tokenomics.GenesisState{}, false
	}
	return *c.genesisState, true
}
func (c *Chain) GenesisSupply() uint64  { c.mu.RLock(); defer c.mu.RUnlock(); return c.genesisSupply }
func (c *Chain) MiningIssued() uint64   { c.mu.RLock(); defer c.mu.RUnlock(); return c.miningIssued }
func (c *Chain) IsPhase7() bool         { c.mu.RLock(); defer c.mu.RUnlock(); return c.genesisState != nil }
func (c *Chain) IsV2() bool             { c.mu.RLock(); defer c.mu.RUnlock(); return c.protocolVersion == 2 }
func (c *Chain) UsesV2Rules() bool      { c.mu.RLock(); defer c.mu.RUnlock(); return c.protocolVersion >= 2 }
func (c *Chain) StateRoot() string      { c.mu.RLock(); defer c.mu.RUnlock(); return c.stateRoot }
func (c *Chain) ProtocolVersion() uint8 { c.mu.RLock(); defer c.mu.RUnlock(); return c.protocolVersion }
func (c *Chain) NetworkID() string      { c.mu.RLock(); defer c.mu.RUnlock(); return c.networkID }
func (c *Chain) NetworkName() string    { c.mu.RLock(); defer c.mu.RUnlock(); return c.networkName }
func (c *Chain) NextNonce(address string) uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nonces[address]
}

func (c *Chain) HeaderBytesAfter(locator [][32]byte, stop [32]byte, max int) [][]byte { return nil }
func (c *Chain) ActiveHeaderHashes() [][32]byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([][32]byte, 0, len(c.active))
	for _, b := range c.active {
		var h [32]byte
		raw := []byte(b.Hash)
		if len(raw) == 64 {
			if _, err := hex.Decode(h[:], raw); err != nil {
				h = [32]byte{}
			}
		}
		out = append(out, h)
	}
	return out
}
func (c *Chain) FindCommonLocator(loc [][32]byte) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, lh := range loc {
		h := fmt.Sprintf("%x", lh[:])
		for i := len(c.active) - 1; i >= 0; i-- {
			if c.active[i].Hash == h {
				return i, true
			}
		}
	}
	return 0, false
}
func (c *Chain) HeadersFromLocator(loc [][32]byte, stop [32]byte, max int) []*block.Block {
	c.mu.RLock()
	defer c.mu.RUnlock()
	idx, found := 0, false
	for _, lh := range loc {
		h := fmt.Sprintf("%x", lh[:])
		for i := len(c.active) - 1; i >= 0; i-- {
			if c.active[i].Hash == h {
				idx = i
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	start := idx + 1
	if !found {
		start = 1
	}
	out := make([]*block.Block, 0, max)
	for i := start; i < len(c.active) && len(out) < max; i++ {
		out = append(out, c.active[i])
		if stop != ([32]byte{}) && c.active[i].Hash == fmt.Sprintf("%x", stop[:]) {
			break
		}
	}
	return out
}
func (c *Chain) BlocksByHashes(hashes [][32]byte) []*block.Block {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*block.Block, 0, len(hashes))
	for _, h := range hashes {
		if b, ok := c.blocks[fmt.Sprintf("%x", h[:])]; ok {
			out = append(out, b)
		}
	}
	return out
}
func (c *Chain) BalanceSnapshot() map[string]uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := make(map[string]uint64, len(c.balances))
	for k, v := range c.balances {
		m[k] = v
	}
	return m
}
