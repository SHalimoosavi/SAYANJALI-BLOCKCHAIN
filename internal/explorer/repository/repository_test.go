package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/explorer/model"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/explorer/schema"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/pkg/protocol"
)

func openTestRepo(t *testing.T) *Repository {
	t.Helper()

	r, err := New(
		context.Background(),
		filepath.Join(t.TempDir(), "explorer.db"),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = r.Close()
	})

	return r
}

func txV1(
	hash,
	sender,
	receiver string,
	amount uint64,
) model.Transaction {
	return model.Transaction{
		Version:         model.V1Version,
		NetworkID:       "sayanjali-mainnet-mvp",
		Nonce:           1,
		Sender:          sender,
		Receiver:        receiver,
		AmountBaseUnits: amount,
		TxHash:          hash,
	}
}

func txV2(
	id,
	sender,
	receiver string,
	amount uint64,
) model.Transaction {
	return model.Transaction{
		Version:         model.V2Version,
		NetworkID:       "sayanjali-mainnet-mvp",
		Nonce:           1,
		Sender:          sender,
		Receiver:        receiver,
		AmountBaseUnits: amount,
		TxID:            id,
	}
}

func block(
	hash,
	prev string,
	h int64,
	txs ...model.Transaction,
) model.Block {
	return model.Block{
		Hash:         hash,
		Height:       h,
		PreviousHash: prev,
		Nonce:        uint64(h),
		Difficulty:   1,
		MerkleRoot:   "mr",
		ChainWork:    "1",
		Transactions: txs,
	}
}

func TestFreshMigrateAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	ctx := context.Background()

	r, err := New(ctx, path)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	r, err = New(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	var version int
	if err := r.Reader().QueryRow(`
		SELECT MAX(version)
		FROM schema_migrations
	`).Scan(&version); err != nil {
		t.Fatal(err)
	}

	if version != 1 {
		t.Fatalf("version=%d", version)
	}
}

func TestDuplicateBlockInsertIsIdempotent(t *testing.T) {
	r := openTestRepo(t)

	b := block(
		"g",
		"",
		0,
		txV1("tx1", "A", "B", 10),
	)

	if err := r.InsertBlock(context.Background(), b); err != nil {
		t.Fatal(err)
	}

	if err := r.InsertBlock(context.Background(), b); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := r.Reader().QueryRow(`
		SELECT COUNT(*)
		FROM blocks
	`).Scan(&n); err != nil {
		t.Fatal(err)
	}

	if n != 1 {
		t.Fatalf("blocks=%d", n)
	}
}

func TestOrphansSameHeight(t *testing.T) {
	r := openTestRepo(t)

	for _, b := range []model.Block{
		block("a", "p", 5),
		block("b", "q", 5),
	} {
		if err := r.InsertBlock(context.Background(), b); err != nil {
			t.Fatal(err)
		}
	}

	var n int
	if err := r.Reader().QueryRow(`
		SELECT COUNT(*)
		FROM blocks
		WHERE height=5
	`).Scan(&n); err != nil {
		t.Fatal(err)
	}

	if n != 2 {
		t.Fatalf("height rows=%d", n)
	}
}

func TestCanonicalFlipOrderAndReorgAggregates(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	gen := block(
		"g",
		"",
		0,
		txV1(
			"g-tx",
			protocol.GenesisAllocationSender,
			"A",
			100,
		),
	)

	old := block(
		"old",
		"g",
		1,
		txV1("old-tx", "A", "B", 30),
	)

	newb := block(
		"new",
		"g",
		1,
		txV2("new-tx", "A", "C", 20),
	)

	for _, b := range []model.Block{gen, old, newb} {
		if err := r.InsertBlock(ctx, b); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.SetCanonical(ctx, []string{"g", "old"}); err != nil {
		t.Fatal(err)
	}

	var a, b, c uint64

	for _, q := range []struct {
		addr string
		p    *uint64
	}{
		{"A", &a},
		{"B", &b},
		{"C", &c},
	} {
		var balance int64
		if err := r.Reader().QueryRow(`
			SELECT balance
			FROM addresses
			WHERE address=?
		`, q.addr).Scan(&balance); err != nil {
			t.Fatal(err)
		}
		*q.p = uint64(balance)
	}

	if a != 70 || b != 30 || c != 0 {
		t.Fatalf(
			"first canonical balances A=%d B=%d C=%d",
			a,
			b,
			c,
		)
	}

	if err := r.SetCanonical(ctx, []string{"g", "new"}); err != nil {
		t.Fatal(err)
	}

	a, b, c = 0, 0, 0

	for _, q := range []struct {
		addr string
		p    *uint64
	}{
		{"A", &a},
		{"B", &b},
		{"C", &c},
	} {
		var balance int64
		if err := r.Reader().QueryRow(`
			SELECT balance
			FROM addresses
			WHERE address=?
		`, q.addr).Scan(&balance); err != nil {
			t.Fatal(err)
		}
		*q.p = uint64(balance)
	}

	if a != 80 || b != 0 || c != 20 {
		t.Fatalf(
			"reorg balances A=%d B=%d C=%d",
			a,
			b,
			c,
		)
	}

	if err := r.CheckInvariants(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Writer().Exec(`
		UPDATE addresses
		SET balance=999
		WHERE address='A'
	`); err != nil {
		t.Fatal(err)
	}

	if err := r.CheckInvariants(ctx); err == nil {
		t.Fatal("expected invariant failure")
	}
}

func TestInitialBalanceRecomputeWhenReceivedIsLessThanSent(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	b := block(
		"g",
		"",
		0,
		txV1("tx", "A", "B", 100),
	)

	if err := r.InsertBlock(ctx, b); err != nil {
		t.Fatal(err)
	}

	if err := r.SetInitialBalance(ctx, "A", 500); err != nil {
		t.Fatal(err)
	}

	if err := r.SetCanonical(ctx, []string{"g"}); err != nil {
		t.Fatal(err)
	}

	var balance int64
	if err := r.Reader().QueryRow(`
		SELECT balance
		FROM addresses
		WHERE address='A'
	`).Scan(&balance); err != nil {
		t.Fatal(err)
	}

	if balance != 400 {
		t.Fatalf("balance=%d", balance)
	}
}

func TestSetInitialBalanceRecomputesHistory(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	b := block(
		"g",
		"",
		0,
		txV1("tx", "A", "B", 100),
	)

	if err := r.InsertBlock(ctx, b); err != nil {
		t.Fatal(err)
	}

	if err := r.SetInitialBalance(ctx, "A", 500); err != nil {
		t.Fatal(err)
	}

	if err := r.SetCanonical(ctx, []string{"g"}); err != nil {
		t.Fatal(err)
	}

	if err := r.SetInitialBalance(ctx, "A", 700); err != nil {
		t.Fatal(err)
	}

	var balance int64
	if err := r.Reader().QueryRow(`
		SELECT balance
		FROM addresses
		WHERE address='A'
	`).Scan(&balance); err != nil {
		t.Fatal(err)
	}

	if balance != 600 {
		t.Fatalf("balance=%d", balance)
	}
}

func TestCoinbaseAndGenesisAllocationDoNotCreateOutRows(
	t *testing.T,
) {
	r := openTestRepo(t)
	ctx := context.Background()

	coinbase := txV2(
		"coinbase",
		protocol.CoinbaseSender,
		"A",
		5,
	)

	genesisAllocation := txV2(
		"genesis",
		protocol.GenesisAllocationSender,
		"B",
		7,
	)

	if err := r.InsertBlock(
		ctx,
		block("g", "", 0, coinbase, genesisAllocation),
	); err != nil {
		t.Fatal(err)
	}

	var outRows int
	if err := r.Reader().QueryRow(`
		SELECT COUNT(*)
		FROM address_transactions
		WHERE direction='out'
		  AND address IN (?, ?)
	`,
		protocol.CoinbaseSender,
		protocol.GenesisAllocationSender,
	).Scan(&outRows); err != nil {
		t.Fatal(err)
	}

	if outRows != 0 {
		t.Fatalf("issuance out rows=%d", outRows)
	}

	var coinbaseAddressRows int
	if err := r.Reader().QueryRow(`
		SELECT COUNT(*)
		FROM addresses
		WHERE address=?
	`, protocol.CoinbaseSender).Scan(&coinbaseAddressRows); err != nil {
		t.Fatal(err)
	}

	if coinbaseAddressRows != 0 {
		t.Fatalf(
			"coinbase sentinel address rows=%d",
			coinbaseAddressRows,
		)
	}

	var genesisAddressRows int
	if err := r.Reader().QueryRow(`
		SELECT COUNT(*)
		FROM addresses
		WHERE address=?
	`, protocol.GenesisAllocationSender).Scan(&genesisAddressRows); err != nil {
		t.Fatal(err)
	}

	if genesisAddressRows != 0 {
		t.Fatalf(
			"genesis allocation sentinel address rows=%d",
			genesisAddressRows,
		)
	}

	var emptySenderRows int
	if err := r.Reader().QueryRow(`
		SELECT COUNT(*)
		FROM addresses
		WHERE address=''
	`).Scan(&emptySenderRows); err != nil {
		t.Fatal(err)
	}

	if emptySenderRows != 0 {
		t.Fatalf("empty sender address rows=%d", emptySenderRows)
	}
}

func TestEmptySenderDoesNotCreateOutRow(t *testing.T) {
	r := openTestRepo(t)

	tx := txV1(
		"empty-sender",
		"",
		"A",
		5,
	)

	if err := r.InsertBlock(
		context.Background(),
		block("g", "", 0, tx),
	); err != nil {
		t.Fatal(err)
	}

	var emptyRows int
	if err := r.Reader().QueryRow(`
		SELECT COUNT(*)
		FROM address_transactions
		WHERE address=''
		  AND direction='out'
	`).Scan(&emptyRows); err != nil {
		t.Fatal(err)
	}

	if emptyRows != 0 {
		t.Fatalf("empty sender out rows=%d", emptyRows)
	}
}

func TestCanonicalSetRejectsNonContiguousChain(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	for _, b := range []model.Block{
		block("g", "", 0),
		block("b1", "g", 1),
		block("b3", "b1", 3),
	} {
		if err := r.InsertBlock(ctx, b); err != nil {
			t.Fatal(err)
		}
	}

	err := r.SetCanonical(ctx, []string{"g", "b1", "b3"})
	if err == nil {
		t.Fatal("expected non-contiguous canonical set rejection")
	}

	if !strings.Contains(err.Error(), "non-contiguous") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCanonicalSetRejectsDuplicateHash(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	for _, b := range []model.Block{
		block("g", "", 0),
		block("b1", "g", 1),
	} {
		if err := r.InsertBlock(ctx, b); err != nil {
			t.Fatal(err)
		}
	}

	err := r.SetCanonical(ctx, []string{"g", "b1", "b1"})
	if err == nil {
		t.Fatal("expected duplicate canonical hash rejection")
	}

	if !strings.Contains(err.Error(), "duplicate canonical block hash") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCanonicalSetRejectsWrongPreviousHash(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	for _, b := range []model.Block{
		block("g", "", 0),
		block("b1", "g", 1),
		block("b2", "wrong", 2),
	} {
		if err := r.InsertBlock(ctx, b); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.SetCanonical(
		ctx,
		[]string{"g", "b1", "b2"},
	); err == nil {
		t.Fatal("expected previous_hash rejection")
	}
}

func TestInvariantCheckerCatchesMissingCanonicalParent(
	t *testing.T,
) {
	r := openTestRepo(t)
	ctx := context.Background()

	if err := r.InsertBlock(
		ctx,
		block("b1", "missing-parent", 1),
	); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Writer().Exec(`
		UPDATE blocks
		SET is_canonical=1
		WHERE hash='b1'
	`); err != nil {
		t.Fatal(err)
	}

	if err := r.CheckInvariants(ctx); err == nil {
		t.Fatal("expected missing canonical parent invariant failure")
	}
}

func TestSetCanonicalAloneKeepsAggregatesCorrectAcrossReorg(
	t *testing.T,
) {
	r := openTestRepo(t)
	ctx := context.Background()

	gen := block(
		"g",
		"",
		0,
		txV2(
			"genesis",
			protocol.GenesisAllocationSender,
			"A",
			100,
		),
	)

	old := block(
		"old",
		"g",
		1,
		txV2("old-transfer", "A", "B", 30),
	)

	newb := block(
		"new",
		"g",
		1,
		txV2(
			"new-coinbase",
			protocol.CoinbaseSender,
			"C",
			50,
		),
		txV2("self-transfer", "A", "A", 10),
	)

	for _, b := range []model.Block{gen, old, newb} {
		if err := r.InsertBlock(ctx, b); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.SetCanonical(ctx, []string{"g", "old"}); err != nil {
		t.Fatal(err)
	}

	var balanceA, balanceB int64

	if err := r.Reader().QueryRow(`
		SELECT balance FROM addresses WHERE address='A'
	`).Scan(&balanceA); err != nil {
		t.Fatal(err)
	}

	if err := r.Reader().QueryRow(`
		SELECT balance FROM addresses WHERE address='B'
	`).Scan(&balanceB); err != nil {
		t.Fatal(err)
	}

	if balanceA != 70 || balanceB != 30 {
		t.Fatalf(
			"old balances A=%d B=%d",
			balanceA,
			balanceB,
		)
	}

	if err := r.SetCanonical(ctx, []string{"g", "new"}); err != nil {
		t.Fatal(err)
	}

	var (
		a int64
		c int64
	)

	if err := r.Reader().QueryRow(`
		SELECT balance FROM addresses WHERE address='A'
	`).Scan(&a); err != nil {
		t.Fatal(err)
	}

	if err := r.Reader().QueryRow(`
		SELECT balance FROM addresses WHERE address='C'
	`).Scan(&c); err != nil {
		t.Fatal(err)
	}

	if a != 100 || c != 50 {
		t.Fatalf(
			"reorg balances A=%d C=%d",
			a,
			c,
		)
	}

	if err := r.CheckInvariants(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestTxCountUsesDistinctTransactionIdentities(t *testing.T) {
	r := openTestRepo(t)

	if err := r.InsertBlock(
		context.Background(),
		block(
			"g",
			"",
			0,
			txV1("tx1", "A", "B", 1),
			txV2("tx2", "B", "C", 2),
		),
	); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := r.Reader().QueryRow(`
		SELECT tx_count
		FROM blocks
		WHERE hash='g'
	`).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 2 {
		t.Fatalf("tx_count=%d", count)
	}
}

func TestInvariantCheckerCatchesAggregateMismatch(
	t *testing.T,
) {
	r := openTestRepo(t)
	ctx := context.Background()

	if err := r.InsertBlock(
		ctx,
		block("g", "", 0, txV1("tx", "A", "B", 1)),
	); err != nil {
		t.Fatal(err)
	}

	if err := r.SetInitialBalance(ctx, "A", 1); err != nil {
		t.Fatal(err)
	}

	if err := r.SetCanonical(ctx, []string{"g"}); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Writer().Exec(`
		UPDATE addresses
		SET balance = balance + 1
		WHERE address = 'A'
	`); err != nil {
		t.Fatal(err)
	}

	if err := r.CheckInvariants(ctx); err == nil {
		t.Fatal("expected aggregate mismatch invariant failure")
	}
}

func TestPaginationClampsAndIsBounded(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	for i := int64(0); i < 120; i++ {
		prev := ""
		if i > 0 {
			prev = fmt.Sprintf("b%d", i-1)
		}

		if err := r.InsertBlock(
			ctx,
			block(fmt.Sprintf("b%d", i), prev, i),
		); err != nil {
			t.Fatal(err)
		}
	}

	hashes := make([]string, 120)
	for i := range hashes {
		hashes[i] = fmt.Sprintf("b%d", i)
	}

	if err := r.SetCanonical(ctx, hashes); err != nil {
		t.Fatal(err)
	}

	rows, err := r.GetBlocks(ctx, 0, 119, 1000, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 100 {
		t.Fatalf("rows=%d", len(rows))
	}

	rows, err = r.GetBlocks(ctx, 0, 119, 0, -10)
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 1 {
		t.Fatalf("clamped rows=%d", len(rows))
	}
}

func TestResetToCheckpointPreservesInitialBalancesAndRecomputes(
	t *testing.T,
) {
	r := openTestRepo(t)
	ctx := context.Background()

	gen := block(
		"g",
		"",
		0,
		txV1(
			"g-tx",
			protocol.GenesisAllocationSender,
			"A",
			100,
		),
	)

	b1 := block(
		"b1",
		"g",
		1,
		txV1("t1", "A", "B", 20),
	)

	b2 := block(
		"b2",
		"b1",
		2,
		txV1("t2", "A", "C", 10),
	)

	for _, b := range []model.Block{gen, b1, b2} {
		if err := r.InsertBlock(ctx, b); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.SetCanonical(
		ctx,
		[]string{"g", "b1", "b2"},
	); err != nil {
		t.Fatal(err)
	}

	if err := r.SetInitialBalance(ctx, "A", 500); err != nil {
		t.Fatal(err)
	}

	var beforeA, beforeB int64

	if err := r.Reader().QueryRow(`
		SELECT balance FROM addresses WHERE address='A'
	`).Scan(&beforeA); err != nil {
		t.Fatal(err)
	}

	if err := r.Reader().QueryRow(`
		SELECT balance FROM addresses WHERE address='B'
	`).Scan(&beforeB); err != nil {
		t.Fatal(err)
	}

	if err := r.ResetToCheckpoint(
		ctx,
		1,
		"b1",
	); err != nil {
		t.Fatal(err)
	}

	var afterA, afterB int64

	if err := r.Reader().QueryRow(`
		SELECT balance FROM addresses WHERE address='A'
	`).Scan(&afterA); err != nil {
		t.Fatal(err)
	}

	if err := r.Reader().QueryRow(`
		SELECT balance FROM addresses WHERE address='B'
	`).Scan(&afterB); err != nil {
		t.Fatal(err)
	}

	if beforeA != 570 ||
		beforeB != 20 ||
		afterA != 580 ||
		afterB != 20 {
		t.Fatalf(
			"balances before A=%d B=%d after A=%d B=%d",
			beforeA,
			beforeB,
			afterA,
			afterB,
		)
	}

	var initial int64

	if err := r.Reader().QueryRow(`
		SELECT initial_balance
		FROM addresses
		WHERE address='A'
	`).Scan(&initial); err != nil {
		t.Fatal(err)
	}

	if initial != 500 {
		t.Fatalf("initial balance=%d", initial)
	}

	if err := r.CheckInvariants(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestOverflowGuard(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	if err := r.SetInitialBalance(
		ctx,
		"A",
		math.MaxUint64,
	); err == nil {
		t.Fatal("expected SQLite range rejection")
	}

	if err := r.SetInitialBalance(
		ctx,
		"A",
		math.MaxInt64,
	); err != nil {
		t.Fatal(err)
	}

	b := block(
		"g",
		"",
		0,
		txV1("tx", "B", "A", 1),
	)

	if err := r.InsertBlock(ctx, b); err != nil {
		t.Fatal(err)
	}

	if err := r.SetCanonical(ctx, []string{"g"}); err == nil {
		t.Fatal("expected canonical update to reject aggregate overflow")
	}

	var canonical int
	if err := r.Reader().QueryRow(`
		SELECT is_canonical FROM blocks WHERE hash='g'
	`).Scan(&canonical); err != nil {
		t.Fatal(err)
	}

	if canonical != 0 {
		t.Fatalf("block became canonical after rejected overflow: is_canonical=%d", canonical)
	}
}

func TestForeignKeysEnabledOnReader(t *testing.T) {
	r := openTestRepo(t)

	var fk int
	if err := r.Reader().QueryRow(`
		PRAGMA foreign_keys
	`).Scan(&fk); err != nil {
		t.Fatal(err)
	}

	if fk != 1 {
		t.Fatalf("foreign_keys=%d", fk)
	}
}

func TestInvariantCheckerCatchesCorruptedCanonicalParent(
	t *testing.T,
) {
	r := openTestRepo(t)
	ctx := context.Background()

	for _, b := range []model.Block{
		block("g", "", 0),
		block("b1", "g", 1),
	} {
		if err := r.InsertBlock(ctx, b); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.SetCanonical(
		ctx,
		[]string{"g", "b1"},
	); err != nil {
		t.Fatal(err)
	}

	if err := r.CheckInvariants(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Writer().Exec(`
		UPDATE blocks
		SET is_canonical=0
		WHERE hash='g'
	`); err != nil {
		t.Fatal(err)
	}

	if err := r.CheckInvariants(ctx); err == nil {
		t.Fatal("expected canonical parent invariant failure")
	}
}

func TestReadWhileWriterTransactionOpen(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	if err := r.InsertBlock(
		ctx,
		block("g", "", 0),
	); err != nil {
		t.Fatal(err)
	}

	tx, err := r.Writer().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO network_snapshots(
			height,tip_hash,observed_at,peer_count,payload_json
		)
		VALUES(0,'g',0,0,'{}')
	`); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := r.Reader().QueryRow(`
		SELECT COUNT(*) FROM blocks
	`).Scan(&n); err != nil {
		t.Fatal(err)
	}

	if n != 1 {
		t.Fatalf("blocks=%d", n)
	}
}

func TestGenesisAllocationSenderConstantIsAuthoritative(
	t *testing.T,
) {
	r := openTestRepo(t)

	tx := txV2(
		"genesis-allocation",
		protocol.GenesisAllocationSender,
		"A",
		1,
	)

	if err := r.InsertBlock(
		context.Background(),
		block("g", "", 0, tx),
	); err != nil {
		t.Fatal(err)
	}
}

func TestV1V2IdentityAndCoinbase(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	v1 := txV1("v1", "A", "B", 1)
	v2 := txV2("v2", "A", "C", 2)

	cb := model.Transaction{
		Version:         model.V2Version,
		NetworkID:       "sayanjali-mainnet-mvp",
		Sender:          protocol.CoinbaseSender,
		Receiver:        "D",
		AmountBaseUnits: 5,
		TxID:            "cb",
	}

	if err := r.InsertBlock(
		ctx,
		block("g", "", 0, v1, v2, cb),
	); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := r.Reader().QueryRow(`
		SELECT COUNT(*) FROM transactions
	`).Scan(&n); err != nil {
		t.Fatal(err)
	}

	if n != 3 {
		t.Fatalf("txs=%d", n)
	}
}

func TestMigrateNoOp(t *testing.T) {
	r := openTestRepo(t)
	ctx := context.Background()

	if err := schema.Migrate(ctx, r.Writer()); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := r.Reader().QueryRow(`
		SELECT COUNT(*) FROM schema_migrations
	`).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("migration rows=%d", count)
	}
}

var _ = sql.ErrNoRows
