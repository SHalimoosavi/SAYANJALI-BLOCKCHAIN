package chain

import (
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/consensus"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/networkid"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/tokenomics"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/wallet"
)

func v2TestState(t *testing.T) (tokenomics.GenesisState, *wallet.KeyPair, string) {
	t.Helper()
	k, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}
	gs, err := tokenomics.Load("../../configs/genesis/phase7-private-testnet.state.json")
	if err != nil {
		t.Fatal(err)
	}
	gs.Allocations[0].Recipient = k.Address
	commitment, err := gs.Commitment()
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := networkid.EffectiveNetworkID(networkid.Inputs{
		ConsensusProtocolVersion: 2,
		GenesisStateCommitment:   commitment,
		HistoricalGenesisHash:    "5c95d2d7b63bde94fbfcfe4c2a95850ca96e0fc649e93e545b8b32f3a9c7cc1b",
		NetworkName:              tokenomics.Phase7NetworkName,
	})
	if err != nil {
		t.Fatal(err)
	}
	return gs, k, id
}

func v2Tx(t *testing.T, k *wallet.KeyPair, network string, nonce uint64, amount uint64) transaction.Transaction {
	t.Helper()
	tx := transaction.Transaction{Version: 2, NetworkID: network, Sender: k.Address, Receiver: "SYJ1111111111111111111111111111111111111111", AmountBaseUnits: amount, Timestamp: 1735689601.25 + float64(nonce), Nonce: nonce}
	if err := tx.SignV2(k, network); err != nil {
		t.Fatal(err)
	}
	return tx
}

func TestV2NonceSequenceAndReplayProtection(t *testing.T) {
	gs, k, network := v2TestState(t)
	g, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}
	reward, _ := consensus.ExpectedMiningReward(0)

	tx0 := v2Tx(t, k, network, 0, 1)
	b1, err := func() (*block.Block, error) {
		coin, e := transaction.NewV2Coinbase("SYJ1111111111111111111111111111111111111111", network, reward, g.Timestamp+1)
		if e != nil {
			return nil, e
		}
		return block.New(1, g.Hash, g.Timestamp+1, 0, 4, []transaction.Transaction{coin, tx0})
	}()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateV2TransactionsAndState([]*block.Block{g}, b1, &gs, network, reward); err != nil {
		t.Fatal(err)
	}

	tx1 := v2Tx(t, k, network, 1, 1)
	b2, err := func() (*block.Block, error) {
		coin, e := transaction.NewV2Coinbase("SYJ1111111111111111111111111111111111111111", network, reward, b1.Timestamp+1)
		if e != nil {
			return nil, e
		}
		return block.New(2, b1.Hash, b1.Timestamp+1, 0, 4, []transaction.Transaction{coin, tx1})
	}()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateV2TransactionsAndState([]*block.Block{g, b1}, b2, &gs, network, reward); err != nil {
		t.Fatal(err)
	}

	gap := v2Tx(t, k, network, 3, 1)
	bg, err := func() (*block.Block, error) {
		coin, e := transaction.NewV2Coinbase("SYJ1111111111111111111111111111111111111111", network, reward, b2.Timestamp+1)
		if e != nil {
			return nil, e
		}
		return block.New(3, b2.Hash, b2.Timestamp+1, 0, 4, []transaction.Transaction{coin, gap})
	}()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateV2TransactionsAndState([]*block.Block{g, b1, b2}, bg, &gs, network, reward); err == nil {
		t.Fatal("nonce gap accepted")
	}

	dup, err := func() (*block.Block, error) {
		coin, e := transaction.NewV2Coinbase("SYJ1111111111111111111111111111111111111111", network, reward, b2.Timestamp+1)
		if e != nil {
			return nil, e
		}
		return block.New(3, b2.Hash, b2.Timestamp+1, 0, 4, []transaction.Transaction{coin, tx1, tx1})
	}()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateV2TransactionsAndState([]*block.Block{g, b1, b2}, dup, &gs, network, reward); err == nil {
		t.Fatal("same-block duplicate accepted")
	}

	replay, err := func() (*block.Block, error) {
		coin, e := transaction.NewV2Coinbase("SYJ1111111111111111111111111111111111111111", network, reward, b2.Timestamp+1)
		if e != nil {
			return nil, e
		}
		return block.New(3, b2.Hash, b2.Timestamp+1, 0, 4, []transaction.Transaction{coin, tx0})
	}()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateV2TransactionsAndState([]*block.Block{g, b1, b2}, replay, &gs, network, reward); err == nil {
		t.Fatal("cross-block transaction replay accepted")
	}
}

func TestV2CoinbaseDoesNotAdvanceAccountNonce(t *testing.T) {
	gs, k, network := v2TestState(t)
	g, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}
	reward, _ := consensus.ExpectedMiningReward(0)
	tx0 := v2Tx(t, k, network, 0, 1)
	coin, err := transaction.NewV2Coinbase("SYJ1111111111111111111111111111111111111111", network, reward, g.Timestamp+1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := block.New(1, g.Hash, g.Timestamp+1, 0, 4, []transaction.Transaction{coin, tx0})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateV2TransactionsAndState([]*block.Block{g}, b, &gs, network, reward); err != nil {
		t.Fatal(err)
	}
}
