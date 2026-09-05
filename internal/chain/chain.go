package chain

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/consensus"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/state"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

type Chain struct {
	mu        sync.RWMutex
	store     *storage.Store
	blocks    map[string]*block.Block
	activeTip string
	active    []*block.Block
	balances  state.Balances
	supply    uint64
}

func Open(store *storage.Store) (*Chain, error) {
	c := &Chain{store: store, blocks: make(map[string]*block.Block), balances: make(state.Balances)}
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
		bs = []*block.Block{g}
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
	if err := ValidateChain(candidate); err != nil {
		return nil, err
	}
	c.activeTip = tip
	c.active = candidate
	c.replayState(candidate)
	return c, nil
}
func ValidateChain(ch []*block.Block) error {
	if len(ch) == 0 {
		return errors.New("empty chain")
	}
	if err := block.ValidateGenesis(ch[0]); err != nil {
		return err
	}
	cfg := protocol.DefaultDifficultyConfig()
	for i := 1; i < len(ch); i++ {
		if err := validateNext(ch[i], ch[:i], cfg); err != nil {
			return fmt.Errorf("block %d: %w", ch[i].Index, err)
		}
	}
	return nil
}
func validateNext(b *block.Block, prefix []*block.Block, cfg protocol.DifficultyConfig) error {
	p := prefix[len(prefix)-1]
	if b.Index != p.Index+1 {
		return errors.New("non-sequential block index")
	}
	if b.PreviousHash != p.Hash {
		return errors.New("previous hash mismatch")
	}
	if b.Timestamp <= p.Timestamp {
		return errors.New("timestamp must increase")
	}
	reward, ok := consensus.ExpectedReward(supplyFor(prefix))
	if !ok {
		return errors.New("no issuance remains")
	}
	if err := transactionsValid(b, reward); err != nil {
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
	if err := stateTransition(prefix, b, reward); err != nil {
		return err
	}
	return nil
}

// requiredNextDifficulty applies the frozen retarget algorithm only at a
// complete 10-block epoch boundary. Before then, the previous mineable
// difficulty remains in force. This keeps the Go node from feeding a short
// window into the frozen Python Fraction-based retarget routine.
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
func stateTransition(prefix []*block.Block, b *block.Block, reward uint64) error {
	balances := make(state.Balances)
	var supply uint64
	for _, bl := range prefix {
		if bl.Index == 0 {
			continue
		}
		for _, tx := range bl.Transactions {
			if tx.Sender == protocol.CoinbaseSender {
				if !balancesCredit(balances, tx.Receiver, tx.AmountBaseUnits) {
					return errors.New("balance overflow")
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
	}
	coinbases := 0
	for _, tx := range b.Transactions {
		if tx.Sender == protocol.CoinbaseSender {
			coinbases++
			if tx.AmountBaseUnits != reward {
				return errors.New("invalid coinbase reward")
			}
			if !balancesCredit(balances, tx.Receiver, tx.AmountBaseUnits) {
				return errors.New("balance overflow")
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
	if supply > protocol.MaxSupplyBaseUnits {
		return errors.New("maximum supply exceeded")
	}
	return nil
}
func balancesCredit(b state.Balances, a string, v uint64) bool { return b.Credit(a, v) == nil }

// transactionsValid performs structural transaction/coinbase checks without mutating chain state.
func transactionsValid(b *block.Block, reward uint64) error {
	if b.Index == 0 {
		return block.ValidateGenesis(b)
	}
	coin := 0
	for _, tx := range b.Transactions {
		if err := tx.Validate(); err != nil { // coinbase is validated structurally below because tx.Validate intentionally allows no positive genesis only
			if tx.Sender != protocol.CoinbaseSender {
				return err
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
func (c *Chain) replayState(ch []*block.Block) {
	c.balances = make(state.Balances)
	c.supply = 0
	for _, b := range ch {
		if b.Index == 0 {
			continue
		}
		for _, tx := range b.Transactions {
			if tx.Sender == protocol.CoinbaseSender {
				_ = c.balances.Credit(tx.Receiver, tx.AmountBaseUnits)
				c.supply += tx.AmountBaseUnits
			} else {
				_ = c.balances.Debit(tx.Sender, tx.AmountBaseUnits)
				_ = c.balances.Credit(tx.Receiver, tx.AmountBaseUnits)
			}
		}
	}
}
func work(ch []*block.Block) *big.Int { return consensus.ChainWork(ch) }
func (c *Chain) Accept(b *block.Block) (bool, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.blocks[b.Hash]; ok {
		return false, "duplicate", nil
	}
	if b.Index == 0 {
		return false, "genesis", errors.New("genesis already exists")
	}
	prefix, err := c.buildChain(b.PreviousHash)
	if err != nil {
		return false, "unknown_ancestor", err
	}
	if err := ValidateChain(prefix); err != nil {
		return false, "invalid_ancestor", err
	}
	if err := validateNext(b, prefix, protocol.DefaultDifficultyConfig()); err != nil {
		return false, "invalid", err
	}
	if err := c.store.SaveBlock(b); err != nil {
		return false, "storage", err
	}
	c.blocks[b.Hash] = b
	candidate := append(append([]*block.Block(nil), prefix...), b)
	if work(candidate).Cmp(work(c.active)) > 0 {
		if err := c.store.SetTip(b.Hash); err != nil {
			return false, "storage", err
		}
		c.activeTip = b.Hash
		c.active = candidate
		c.replayState(candidate)
		return true, "best", nil
	}
	return true, "fork", nil
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
func (c *Chain) HeaderBytesAfter(locator [][32]byte, stop [32]byte, max int) [][]byte { return nil }
func (c *Chain) ActiveHeaderHashes() [][32]byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([][32]byte, 0, len(c.active))
	for _, b := range c.active {
		var h [32]byte
		raw := []byte(b.Hash)
		if len(raw) == 64 {
			for i := 0; i < 32; i++ {
				fmt.Sscanf(string(raw[i*2:i*2+2]), "%02x", &h[i])
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
