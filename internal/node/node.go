package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/chain"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/clock"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/consensus"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/genesis"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/identity"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/mempool"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/networkid"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/p2pnode"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

type Config struct {
	DataDir                           string   `json:"data_dir"`
	NetworkName                       string   `json:"network_name"`
	ListenAddress                     string   `json:"listen_address"`
	AdvertisedAddress                 string   `json:"advertised_address"`
	Seeds                             []string `json:"seeds"`
	MaxPeers                          int      `json:"max_peers"`
	APIListenAddress                  string   `json:"api_listen_address"`
	MempoolMax                        int      `json:"mempool_max"`
	LogLevel                          string   `json:"log_level"`
	Phase7GenesisStatePath            string   `json:"phase7_genesis_state_path,omitempty"`
	Phase7GenesisCommitment           string   `json:"phase7_genesis_commitment,omitempty"`
	ProtocolVersion                   uint8    `json:"protocol_version,omitempty"`
	NetworkID                         string   `json:"network_id,omitempty"`
	GenesisNetworkName                string   `json:"genesis_network_name,omitempty"`
	APIAuthToken                      string   `json:"api_auth_token,omitempty"`
	APIUseTLS                         bool     `json:"api_use_tls,omitempty"`
	APIRequireTLS                     bool     `json:"api_require_tls,omitempty"`
	V3SoloBootstrapMining             bool     `json:"v3_solo_bootstrap_mining,omitempty"`
	V3SoloBootstrapClockSanitySeconds int64    `json:"v3_solo_bootstrap_clock_sanity_seconds,omitempty"`
	APITLSCertFile                    string   `json:"api_tls_cert_file,omitempty"`
	APITLSKeyFile                     string   `json:"api_tls_key_file,omitempty"`
	P2PUseTLS                         bool     `json:"p2p_use_tls,omitempty"`
	P2PTLSCertFile                    string   `json:"p2p_tls_cert_file,omitempty"`
	P2PTLSKeyFile                     string   `json:"p2p_tls_key_file,omitempty"`
	P2PTLSCAFile                      string   `json:"p2p_tls_ca_file,omitempty"`
	P2PTLSServerName                  string   `json:"p2p_tls_server_name,omitempty"`
}

func DefaultConfig(dataDir string) Config {
	return Config{
		DataDir: dataDir, NetworkName: "sayanjali-mainnet-mvp", ListenAddress: "127.0.0.1:3030",
		AdvertisedAddress: "127.0.0.1:3030", MaxPeers: 32, APIListenAddress: "127.0.0.1:8080",
		MempoolMax: 1000, LogLevel: "INFO", V3SoloBootstrapClockSanitySeconds: int64(protocol.V3FutureTimeBound / time.Second),
	}
}
func LoadConfig(path string, defaults Config) (Config, error) {
	info, statErr := os.Stat(path)
	if statErr == nil && info.Mode().Perm()&0077 != 0 {
		return defaults, fmt.Errorf("config file %s has insecure permissions %04o; require owner-only access", path, info.Mode().Perm())
	}
	b, e := os.ReadFile(path) // #nosec G304 -- path is the operator-supplied local node configuration path.
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
	if c.V3SoloBootstrapClockSanitySeconds == 0 {
		c.V3SoloBootstrapClockSanitySeconds = defaults.V3SoloBootstrapClockSanitySeconds
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
	if err := os.WriteFile(path, b, 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

type Node struct {
	cfg        Config
	log        *slog.Logger
	id         *identity.Identity
	store      *storage.Store
	chain      *chain.Chain
	pool       *mempool.Pool
	v2pool     *mempool.V2Pool
	net        *p2pnode.Network
	ctx        context.Context
	cancel     context.CancelFunc
	stopOnce   sync.Once
	done       chan struct{}
	running    atomic.Bool
	clock      clock.Clock
	peerMedian clock.PeerMedian
	acceptFn   func(*block.Block) (bool, string, error)
}

func (c Config) ValidateTransport() error {
	if c.APIRequireTLS && !c.APIUseTLS {
		return errors.New("API TLS is required but api_use_tls is disabled")
	}
	if c.APIUseTLS {
		if c.APITLSCertFile == "" || c.APITLSKeyFile == "" {
			return errors.New("API TLS requires certificate and key files")
		}
	}
	if !c.APIUseTLS && !isLoopbackListenAddress(c.APIListenAddress) {
		return errors.New("API must use TLS when listening on a non-loopback address")
	}
	if c.P2PUseTLS {
		if c.P2PTLSCertFile == "" || c.P2PTLSKeyFile == "" || c.P2PTLSCAFile == "" {
			return errors.New("P2P TLS requires certificate, key, and CA files")
		}
	} else if !isLoopbackListenAddress(c.ListenAddress) {
		return errors.New("P2P TLS is mandatory for non-loopback/public transport")
	}
	return nil
}

func isLoopbackListenAddress(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func New(cfg Config) *Node {
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})
	return &Node{cfg: cfg, log: slog.New(h), done: make(chan struct{}), clock: clock.RealClock{}, peerMedian: clock.UnavailablePeerMedian{}}
}
func (n *Node) Start(ctx context.Context) (retErr error) {
	if n.cfg.ProtocolVersion != 0 && n.cfg.ProtocolVersion != 1 && n.cfg.ProtocolVersion != 2 && n.cfg.ProtocolVersion != 3 {
		return errors.New("unsupported consensus protocol version")
	}
	if n.cfg.ProtocolVersion < 2 && !isLoopbackListenAddress(n.cfg.ListenAddress) {
		return errors.New("legacy V1 consensus is restricted to loopback/private development; public nodes require protocol version 2")
	}
	if n.cfg.ProtocolVersion < 2 && strings.HasPrefix(n.cfg.NetworkName, networkid.V2WirePrefix) {
		return errors.New("V2 network identity requires consensus protocol version 2")
	}
	if err := os.MkdirAll(n.cfg.DataDir, 0700); err != nil {
		return err
	}
	if err := n.cfg.ValidateTransport(); err != nil {
		return err
	}
	if err := n.validateV3BootstrapConfig(); err != nil {
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

	keepStoreOpen := false
	defer func() {
		if keepStoreOpen {
			return
		}
		if closeErr := st.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	n.store = st
	var ch *chain.Chain
	if n.cfg.ProtocolVersion >= 2 {
		if n.cfg.Phase7GenesisStatePath == "" || n.cfg.Phase7GenesisCommitment == "" {
			return errors.New("V2 network requires GenesisState path and commitment")
		}
		if n.cfg.GenesisNetworkName == "" {
			n.cfg.GenesisNetworkName = tokenomics.Phase7NetworkName
		}
		gs, err := tokenomics.Load(n.cfg.Phase7GenesisStatePath)
		if err != nil {
			return err
		}
		commitment, err := gs.Commitment()
		if err != nil {
			return err
		}
		if commitment != n.cfg.Phase7GenesisCommitment {
			return errors.New("V2 GenesisState commitment mismatch")
		}
		if n.cfg.GenesisNetworkName == tokenomics.Phase7NetworkName && commitment != "36351980711cd88fe6ff134f0e4e858ee1a4572a9f44b7bde9f57213e7f1eb82" {
			return errors.New("audited Phase 7 private-testnet GenesisState commitment mismatch")
		}
		g, err := genesis.Build()
		if err != nil {
			return err
		}
		var effectiveID string
		if n.cfg.ProtocolVersion == 2 {
			effectiveID, _, err = networkid.EffectiveNetworkID(networkid.Inputs{ConsensusProtocolVersion: 2, GenesisStateCommitment: commitment, HistoricalGenesisHash: g.Hash, NetworkName: n.cfg.GenesisNetworkName})
			if err != nil {
				return err
			}
			if n.cfg.NetworkID != effectiveID {
				return errors.New("V2 EffectiveNetworkID mismatch")
			}
			if n.cfg.GenesisNetworkName == tokenomics.Phase7NetworkName && effectiveID != "237a1934769295c63fe47771a4996b17c3899e52f6bacf79ed7edefec90eaaf3" {
				return errors.New("audited Phase 7 private-testnet EffectiveNetworkID mismatch")
			}
		} else {
			if err := networkid.ValidateHex64(n.cfg.NetworkID); err != nil {
				return fmt.Errorf("V3 network id invalid: %w", err)
			}
			effectiveID = n.cfg.NetworkID
		}
		wire, err := networkid.WireNetworkName(effectiveID)
		if err != nil {
			return err
		}
		if n.cfg.NetworkName != wire {
			return errors.New("protocol 2/3 network name mismatch")
		}
		var opened *chain.Chain
		if n.cfg.ProtocolVersion == 3 {
			opened, err = chain.OpenV3WithGenesisState(st, gs, effectiveID, wire)
		} else {
			opened, err = chain.OpenV2WithGenesisState(st, gs, effectiveID, wire)
		}
		if err != nil {
			return err
		}
		ch = opened
	} else if n.cfg.NetworkName == tokenomics.Phase7NetworkName {
		if n.cfg.Phase7GenesisStatePath == "" {
			return errors.New("phase 7 network requires a genesis state path")
		}
		if n.cfg.Phase7GenesisCommitment == "" {
			return errors.New("phase 7 network requires a genesis state commitment")
		}
		gs, err := tokenomics.Load(n.cfg.Phase7GenesisStatePath)
		if err != nil {
			return err
		}
		commitment, err := gs.Commitment()
		if err != nil {
			return err
		}
		if commitment != n.cfg.Phase7GenesisCommitment {
			return errors.New("phase 7 genesis commitment mismatch")
		}
		opened, openErr := chain.OpenWithGenesisState(st, gs)
		if openErr != nil {
			return openErr
		}
		ch = opened
	} else {
		opened, openErr := chain.Open(st)
		if openErr != nil {
			return openErr
		}
		ch = opened
	}
	if err != nil {
		return err
	}
	g, _ := genesis.Build()
	if ch.TipHash() == "" || g.Hash == "" {
		return errors.New("genesis initialization failure")
	}
	n.chain = ch
	n.acceptFn = ch.Accept
	n.chain.SetClock(n.clock)
	if n.cfg.ProtocolVersion >= 3 && n.cfg.V3SoloBootstrapMining {
		n.log.Warn("WARNING: V3 SOLO/BOOTSTRAP MINING MODE ENABLED",
			"peer_median_required", false,
			"operator_clock_required", true,
			"purpose", "controlled network bootstrap/testing only")
	}
	n.pool = mempool.New(n.cfg.MempoolMax)
	if n.cfg.ProtocolVersion >= 2 {
		n.v2pool = mempool.NewV2(n.cfg.MempoolMax)
	}
	gHash := g.Hash
	n.net = p2pnode.New(p2pnode.Config{ProtocolVersion: n.cfg.ProtocolVersion, NetworkName: n.cfg.NetworkName, GenesisHash: gHash, NodeID: id.NodeID, PublicKeyHex: id.PublicKeyHex, V2Pool: n.v2pool, AdvertisedAddress: n.cfg.AdvertisedAddress, ListenAddress: n.cfg.ListenAddress, Seeds: n.cfg.Seeds, MaxPeers: n.cfg.MaxPeers, Logger: n.log, Identity: *id, UseTLS: n.cfg.P2PUseTLS, TLSCertFile: n.cfg.P2PTLSCertFile, TLSKeyFile: n.cfg.P2PTLSKeyFile, TLSCAFile: n.cfg.P2PTLSCAFile, TLSServerName: n.cfg.P2PTLSServerName, AcceptBlock: n.AcceptBlock}, ch, n.pool)
	n.ctx, n.cancel = context.WithCancel(ctx)
	if err := n.net.Start(n.ctx); err != nil {
		return err
	}
	n.running.Store(true)
	keepStoreOpen = true
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

// Healthy and Err are placeholders for the L-02 fail-stop contract.
func (n *Node) Healthy() bool         { return true }
func (n *Node) Err() error            { return nil }

// AcceptBlock preserves chain acceptance while exposing a narrow test seam.
func (n *Node) AcceptBlock(b *block.Block) (bool, string, error) {
	if n.acceptFn == nil {
		if n.chain == nil {
			return false, "", errors.New("node not started")
		}
		return n.chain.Accept(b)
	}
	return n.acceptFn(b)
}
func (n *Node) Config() Config        { return n.cfg }
func (n *Node) Identity() identity.Identity {
	if n.id == nil {
		return identity.Identity{}
	}
	return *n.id
}
func (n *Node) Chain() *chain.Chain        { return n.chain }
func (n *Node) Mempool() *mempool.Pool     { return n.pool }
func (n *Node) V2Mempool() *mempool.V2Pool { return n.v2pool }
func (n *Node) Network() *p2pnode.Network  { return n.net }
func (n *Node) SubmitTransaction(tx transaction.Transaction) error {
	if n.chain == nil || n.pool == nil {
		return errors.New("node not started")
	}

	if n.chain.UsesV2Rules() {
		if n.v2pool == nil {
			return errors.New("V2 mempool unavailable")
		}
		if err := tx.ValidateV2(n.chain.NetworkID()); err != nil {
			return err
		}
		if n.chain.HasConfirmedTransaction(tx.TxID) {
			return errors.New("transaction already confirmed")
		}
		next, err := n.v2pool.ExpectedNonce(tx.Sender, n.chain.NextNonce(tx.Sender))
		if err != nil {
			return err
		}
		return n.v2pool.Add(tx, next, n.chain.Balance(tx.Sender), n.chain.NetworkID())
	}
	if err := tx.Validate(); err != nil {
		return err
	}
	if n.chain.HasConfirmedTransaction(tx.TxHash) {
		return errors.New("transaction already confirmed")
	}
	return n.pool.Add(tx, n.chain.Balance)
}
func (n *Node) SetClock(clk clock.Clock) {
	if clk == nil {
		clk = clock.RealClock{}
	}
	n.clock = clk
	if n.chain != nil {
		n.chain.SetClock(clk)
	}
}

func (n *Node) validateV3BootstrapConfig() error {
	if n.cfg.ProtocolVersion < 3 {
		return nil
	}
	max := int64(protocol.V3FutureTimeBound / time.Second)
	if n.cfg.V3SoloBootstrapClockSanitySeconds < 1 || n.cfg.V3SoloBootstrapClockSanitySeconds > max {
		return fmt.Errorf("V3 bootstrap clock sanity must be between 1 and %d seconds", max)
	}
	return nil
}

func (n *Node) SetPeerMedianProvider(provider clock.PeerMedian) {
	if provider == nil {
		provider = clock.UnavailablePeerMedian{}
	}
	n.peerMedian = provider
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
		m["protocol_version"] = n.chain.ProtocolVersion()
		if n.chain.UsesV2Rules() {
			m["network_id"] = n.chain.NetworkID()
		}
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
	return MineNextWithClockSafety(ch, receiver, txs, clock.RealClock{}, clock.UnavailablePeerMedian{})
}

func MineNextWithClockSafety(ch *chain.Chain, receiver string, txs []transaction.Transaction, clk clock.Clock, peerMedian clock.PeerMedian) (*block.Block, error) {
	return mineNextWithClockSafety(ch, receiver, txs, clk, peerMedian, false, 0)
}

func MineNextWithBootstrap(ch *chain.Chain, receiver string, txs []transaction.Transaction, clk clock.Clock, peerMedian clock.PeerMedian, bootstrapEnabled bool, sanitySeconds int64) (*block.Block, error) {
	return mineNextWithClockSafety(ch, receiver, txs, clk, peerMedian, bootstrapEnabled, sanitySeconds)
}

func mineNextWithClockSafety(ch *chain.Chain, receiver string, txs []transaction.Transaction, clk clock.Clock, peerMedian clock.PeerMedian, bootstrapEnabled bool, sanitySeconds int64) (*block.Block, error) {
	if clk == nil {
		clk = clock.RealClock{}
	}
	if peerMedian == nil {
		peerMedian = clock.UnavailablePeerMedian{}
	}
	if ch.ProtocolVersion() >= 3 {
		peerTime, ok := peerMedian.MedianTime()
		if !ok {
			if !bootstrapEnabled {
				return nil, errors.New("V3 mining refused: peer median clock unavailable")
			}
			mtp, ok := ch.MedianTimePast()
			if !ok {
				return nil, errors.New("V3 bootstrap mining refused: median-time-past unavailable")
			}
			now := clk.Now()
			maxSanity := time.Duration(sanitySeconds) * time.Second
			if now.Before(mtp) || now.After(mtp.Add(maxSanity)) {
				return nil, fmt.Errorf("V3 bootstrap mining refused because local clock sanity validation failed: local=%s MTP=%s allowed_drift=%s", now.UTC().Format(time.RFC3339), mtp.UTC().Format(time.RFC3339), maxSanity)
			}
		} else {
			delta := clk.Now().Sub(peerTime)
			if delta < 0 {
				delta = -delta
			}
			if delta > protocol.V3MiningClockSkewLimit {
				return nil, fmt.Errorf("V3 mining refused: local clock differs from peer median by %s (> %s)", delta.Round(time.Second), protocol.V3MiningClockSkewLimit)
			}
		}
	}
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
	timestamp := tip.Timestamp + 1
	if ch.ProtocolVersion() >= 3 {
		mtp, ok := ch.MedianTimePast()
		if !ok {
			return nil, errors.New("V3 mining refused: median-time-past unavailable")
		}
		minimum := mtp.Unix() + 1
		now := clk.Now().Unix()
		if now > minimum {
			timestamp = float64(now)
		} else {
			timestamp = float64(minimum)
		}
	}
	var coin transaction.Transaction
	var err error
	if ch.UsesV2Rules() {
		coin, err = transaction.NewV2Coinbase(receiver, ch.NetworkID(), reward, timestamp)
		if err != nil {
			return nil, err
		}
		txs = selectV2Transactions(ch, txs)
	} else {
		coin = transaction.Transaction{Sender: protocol.CoinbaseSender, Receiver: receiver, AmountBaseUnits: reward, Timestamp: timestamp}
	}
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

func selectV2Transactions(ch *chain.Chain, txs []transaction.Transaction) []transaction.Transaction {
	groups := make(map[string][]transaction.Transaction)
	for _, tx := range txs {
		if !tx.IsV2() || tx.Sender == protocol.CoinbaseSender {
			continue
		}
		groups[tx.Sender] = append(groups[tx.Sender], tx)
	}
	senders := make([]string, 0, len(groups))
	for sender := range groups {
		senders = append(senders, sender)
	}
	sort.Strings(senders)
	out := make([]transaction.Transaction, 0, len(txs))
	for _, sender := range senders {
		arr := groups[sender]
		sort.Slice(arr, func(i, j int) bool { return arr[i].Nonce < arr[j].Nonce })
		expected := ch.NextNonce(sender)
		for _, tx := range arr {
			if tx.Nonce != expected {
				break
			}
			out = append(out, tx)
			if expected == ^uint64(0) {
				break
			}
			expected++
		}
	}
	return out
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
