package block

type Header struct {
	Index        int64   `json:"index"`
	PreviousHash string  `json:"previous_hash"`
	Timestamp    float64 `json:"timestamp"`
	Nonce        uint64  `json:"nonce"`
	Difficulty   int     `json:"difficulty"`
	MerkleRoot   string  `json:"merkle_root"`
}
