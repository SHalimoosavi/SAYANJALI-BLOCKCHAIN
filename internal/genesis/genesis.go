package genesis

import (
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	corecrypto "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/crypto"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

func TransactionHash() string      { return corecrypto.SHA256String(protocol.GenesisMessage) }
func Build() (*block.Block, error) { return block.Genesis() }
