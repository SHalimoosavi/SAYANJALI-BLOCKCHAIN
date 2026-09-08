package node

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/chain"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/consensus"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/genesis"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/identity"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/mempool"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/p2pnode"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

type Config struct {
	DataDir                 string   `json:"data_dir"`
	NetworkName             string   `json:"network_name"`
	ListenAddress           string   `json:"listen_address"`
	AdvertisedAddress       string   `json:"advertised_address"`
	Seeds                   []string `json:"seeds"`
	MaxPeers                int      `json:"max_peers"`
	APIListenAddress        string   `json:"api_listen_address"`
	MempoolMax              int      `json:"mempool_max"`
	LogLevel                string   `json:"log_level"`
	Phase7GenesisStatePath  string   `json:"phase7_genesis_state_path,omitempty"`
	Phase7GenesisCommitment string   `json:"phase7_genesis_commitment,omitempty"`
	APIAuthToken            string   `json:"api_auth_token,omitempty"`
}

func DefaultConfig(dataDir string) Config {
	return Config{DataDir: dataDir, NetworkName: "sayanjali-mainnet-mvp", ListenAddress: "127.0.0.1:3030", AdvertisedAddress: "127.0.0.1:3030", MaxPeers: 32, APIListenAddress: "127.0.0.1:8080", MempoolMax: 1000, LogLevel: "INFO"}
}
func LoadConfig(path string, defaults Config) (Config, error) {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return defaults, nil
	}
	if e != nil {
		return defaults, e
	}
	var c Config
	if e = json.Unmarshal(b, &c); e != nil {
		return defaults, e
	}
	if c.DataDir == "" {
		c.DataDir = defaults.DataDir
	}
	if c.NetworkName == "" {
		c.NetworkName = defaults.NetworkName
	}
	if c.ListenAddress == "" {
		c.ListenAddress = defaults.ListenAddress
	}
	if c.AdvertisedAddress == "" {
		c.AdvertisedAddress = c.ListenAddress
	}
	if c.MaxPeers == 0 {
		c.MaxPeers = defaults.MaxPeers
	}
	if c.APIListenAddress == "" {
		c.APIListenAddress = defaults.APIListenAddress
	}
	if c.MempoolMax == 0 {
		c.MempoolMax = defaults.MempoolMax
	}
	if c.LogLevel == "" {
		c.LogLevel = defaults.LogLevel
	}
	return c, nil
}
func SaveDefaultConfig(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, b, 0600)
}

type Node struct {
	cfg      Config
	log      *slog.Logger
	id       *identity.Identity
	store    *storage.Store
	chain    *chain.Chain
	pool     *mempool.Pool
	net      *p2pnode.Network
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopOnce sync.Once
	done     chan struct{}
	running  atomic.Bool
}

func New(cfg Config) *Node {
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})
	return &Node{cfg: cfg, log: slog.New(h), done: make(chan struct{})}
}
func (n *Node) Start(ctx context.Context) error {
	if err := os.MkdirAll(n.cfg.DataDir, 0700); err != nil {
		return err
	}
	id, created, err := identity.LoadOrCreate(filepath.Join(n.cfg.DataDir, "identity"))
	if err != nil {
		return err
	}
	n.id = id
	if created {
		n.log.Info("node identity created", "node_id", id.NodeID)
	} else {
		n.log.Info("node identity loaded", "node_id", id.NodeID)
	}
	st, err := storage.Open(filepath.Join(n.cfg.DataDir, "chain"))
	if err != nil {
		return err
	}
	n.store = st
	var ch *chain.Chain
	if n.cfg.NetworkName == tokenomics.Phase7NetworkName {
		if n.cfg.Phase7GenesisStatePath == "" {
			st.Close()
			return errors.New("Phase 7 network requires a genesis state path")
		}
		if n.cfg.Phase7GenesisCommitment == "" {
			st.Close()
			return errors.New("Phase 7 network requires a genesis state commitment")
		}
		gs, err := tokenomics.Load(n.cfg.Phase7GenesisStatePath)
		if err != nil {
			st.Close()
			return err
		}
		commitment, err := gs.Commitment()
		if err != nil {
			st.Close()
			return err
		}
		if commitment != n.cfg.Phase7GenesisCommitment {
			st.Close()
			return errors.New("Phase 7 genesis commitment mismatch")
		}
		ch, err = chain.OpenWithGenesisState(st, gs)
	} else {
		ch, err = chain.Open(st)
	}
	if err != nil {
		st.Close()
		return err
	}
	g, _ := genesis.Build()
	if ch.TipHash() == "" || g.Hash == "" {
		st.Close()
		return errors.New("genesis initialization failure")
	}
	n.chain = ch
	n.pool = mempool.New(n.cfg.MempoolMax)
	gHash := g.Hash
	n.net = p2pnode.New(p2pnode.Config{NetworkName: n.cfg.NetworkName, GenesisHash: gHash, NodeID: id.NodeID, PublicKeyHex: id.PublicKeyHex, AdvertisedAddress: n.cfg.AdvertisedAddress, ListenAddress: n.cfg.ListenAddress, Seeds: n.cfg.Seeds, MaxPeers: n.cfg.MaxPeers, Logger: n.log, Identity: *id}, ch, n.pool)
	n.ctx, n.cancel = context.WithCancel(ctx)
	if err := n.net.Start(n.ctx); err != nil {
		st.Close()
		return err
	}
	n.running.Store(true)
	n.log.Info("SYJ node started", "height", ch.Height(), "tip", ch.TipHash(), "network", n.cfg.NetworkName)
	return nil
}
func (n *Node) Stop() {
	n.stopOnce.Do(func() {
		n.running.Store(false)
		if n.cancel != nil {
			n.cancel()
		}
		if n.net != nil {
			n.net.Stop()
		}
		if n.store != nil {
			if err := n.store.Close(); err != nil {
				n.log.Error("storage close failed", "error", err)
			}
		}
		n.log.Info("SYJ node stopped")
		close(n.done)
	})
}
func (n *Node) Done() <-chan struct{} { return n.done }
func (n *Node) Config() Config        { return n.cfg }
func (n *Node) Identity() identity.Identity {
	if n.id == nil {
		return identity.Identity{}
	}
	return *n.id
}
func (n *Node) Chain() *chain.Chain       { return n.chain }
func (n *Node) Mempool() *mempool.Pool    { return n.pool }
func (n *Node) Network() *p2pnode.Network { return n.net }
func (n *Node) SubmitTransaction(tx transaction.Transaction) error {
	if n.chain == nil || n.pool == nil {
		return errors.New("node not started")
	}

	if err := tx.Validate(); err != nil {
		return err
	}

	// Reject transactions already confirmed on the active chain.
	// The index is rebuilt during startup and active-chain reorgs.
	if n.chain.HasConfirmedTransaction(tx.TxHash) {
		return errors.New("transaction already confirmed")
	}

	if err := n.pool.Add(tx, n.chain.Balance); err != nil {
		return err
	}

	return nil
}
func (n *Node) Status() map[string]any {
	m := map[string]any{"running": n.running.Load(), "network": n.cfg.NetworkName}
	if n.chain != nil {
		m["height"] = n.chain.Height()
		m["tip_hash"] = n.chain.TipHash()
		m["chain_work"] = n.chain.Work().String()
		m["supply_base_units"] = n.chain.Supply()
		m["genesis_supply_base_units"] = n.chain.GenesisSupply()
		m["mining_issued_base_units"] = n.chain.MiningIssued()
		m["phase7"] = n.chain.IsPhase7()
	}
	if n.id != nil {
		m["node_id"] = n.id.NodeID
		m["address"] = n.id.Address
	}
	if n.net != nil {
		m["peer_count"] = len(n.net.Peers())
	}
	return m
}
func MineNext(ch *chain.Chain, receiver string, txs []transaction.Transaction) (*block.Block, error) {
	tip := ch.Tip()
	var reward uint64
	var ok bool
	if ch.IsPhase7() {
		reward, ok = consensus.ExpectedMiningReward(ch.MiningIssued())
	} else {
		reward, ok = consensus.ExpectedReward(ch.Supply())
	}
	if !ok {
		return nil, errors.New("no reward remains")
	}
	coin := transaction.Transaction{Sender: protocol.CoinbaseSender, Receiver: receiver, AmountBaseUnits: reward, Timestamp: float64(time.Now().UnixNano()) / 1e9}
	all := append([]transaction.Transaction{coin}, txs...)
	d, err := nextDifficulty(ch)
	if err != nil {
		return nil, err
	}
	b, err := block.New(tip.Index+1, tip.Hash, coin.Timestamp, 0, d, all)
	if err != nil {
		return nil, err
	}
	for b.Nonce < ^uint64(0) {
		if err := b.Recompute(); err != nil {
			return nil, err
		}
		if b.MeetsDifficulty(d) {
			return b, nil
		}
		b.Nonce++
	}
	return nil, errors.New("nonce exhausted")
}
func nextDifficulty(ch *chain.Chain) (int, error) {
	prefix := ch.ChainCopy()
	cfg := protocol.DefaultDifficultyConfig()
	// The frozen genesis block has no mining difficulty field (0).
	// The first mineable block starts at the protocol configuration difficulty.
	base := 4
	if len(prefix) > 0 && prefix[len(prefix)-1].Index > 0 {
		base = prefix[len(prefix)-1].Difficulty
	}
	// The frozen retarget algorithm uses a 10-block window (9 intervals).
	// Before a complete window exists, difficulty remains at the previous
	// block's difficulty. Retargets occur only when the next block starts a
	// new 10-block epoch, so we never feed a fractional/short window into the
	// frozen Fraction-compatible routine.
	nextIndex := int64(len(prefix))
	if len(prefix) < protocol.DifficultyInterval || nextIndex%protocol.DifficultyInterval != 0 {
		return base, nil
	}
	recent := prefix[len(prefix)-protocol.DifficultyInterval:]
	return consensusNext(recent, cfg)
}
func consensusNext(recent []*block.Block, cfg protocol.DifficultyConfig) (int, error) {
	return consensus.NextDifficulty(recent, cfg, 4)
}
