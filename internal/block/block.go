package block

import (
	"errors"
	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/transaction"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

type Block struct {
	Header
	Transactions []transaction.Transaction `json:"transactions"`
	Hash         string                    `json:"hash"`
}

func New(index int64, previousHash string, timestamp float64, nonce uint64, difficulty int, txs []transaction.Transaction) (*Block, error) {
	// Constructors may receive freshly-created transactions whose TxHash has
	// not yet been materialized. The frozen hash algorithm is unchanged: we
	// only populate the canonical transaction hash when it is absent. Existing
	// hashes are preserved so malformed/inconsistent transactions remain
	// detectable by validation. Copy the slice so construction does not mutate
	// the caller's transaction collection.
	normalized := append([]transaction.Transaction(nil), txs...)
	for i := range normalized {
		if normalized[i].TxHash == "" {
			h, err := normalized[i].ComputeHash()
			if err != nil {
				return nil, err
			}
			normalized[i].TxHash = h
		}
	}
	txs = normalized
	hashes := make([]string, len(txs))
	for i := range txs {
		hashes[i] = txs[i].TxHash
	}
	b := &Block{Header: Header{Index: index, PreviousHash: previousHash, Timestamp: timestamp, Nonce: nonce, Difficulty: difficulty, MerkleRoot: MerkleRoot(hashes)}, Transactions: txs}
	h, _, err := HashHeader(b.Header)
	if err != nil {
		return nil, err
	}
	b.Hash = h
	return b, nil
}

func (b *Block) Recompute() error {
	hashes := make([]string, len(b.Transactions))
	for i := range b.Transactions {
		hashes[i] = b.Transactions[i].TxHash
	}
	b.MerkleRoot = MerkleRoot(hashes)
	h, _, err := HashHeader(b.Header)
	if err == nil {
		b.Hash = h
	}
	return err
}
func (b *Block) MeetsDifficulty(d int) bool {
	if d < 0 {
		return false
	}
	if len(b.Hash) < d {
		return false
	}
	for i := 0; i < d; i++ {
		if b.Hash[i] != '0' {
			return false
		}
	}
	return true
}
func Genesis() (*Block, error) {
	txHash := corecrypto.SHA256String(protocol.GenesisMessage)
	tx := transaction.Transaction{Sender: "SYJ-GENESIS-0000000000000000000000000000", Receiver: "SYJ-GENESIS-0000000000000000000000000000", AmountBaseUnits: 0, Timestamp: protocol.GenesisTimestamp, TxHash: txHash}
	return New(0, protocol.GenesisPreviousHash, protocol.GenesisTimestamp, 0, 0, []transaction.Transaction{tx})
}
func ValidateGenesis(b *Block) error {
	g, err := Genesis()
	if err != nil {
		return err
	}
	if b.Hash != g.Hash || b.MerkleRoot != g.MerkleRoot || b.PreviousHash != g.PreviousHash || b.Timestamp != g.Timestamp || b.Nonce != g.Nonce || b.Difficulty != g.Difficulty {
		return errors.New("genesis block does not match frozen protocol identity")
	}
	return nil
}
