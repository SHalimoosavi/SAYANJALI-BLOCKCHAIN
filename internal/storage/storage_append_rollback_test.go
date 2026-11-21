package storage

import (
	"errors"
	"strings"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
)

var (
	errInjectedPayloadWrite = errors.New("injected payload write failure")
	errInjectedTruncate     = errors.New("injected truncate failure")
	errInjectedSync         = errors.New("injected sync failure")
)

type faultJournalFile struct {
	journalFile
	failPayloadWrite bool
	failTruncate     bool
	failSync         bool
	writes           int
}

func (f *faultJournalFile) Write(p []byte) (int, error) {
	if f.failPayloadWrite && f.writes == 1 {
		f.writes++
		return 0, errInjectedPayloadWrite
	}
	f.writes++
	return f.journalFile.Write(p)
}

func (f *faultJournalFile) Truncate(size int64) error {
	if f.failTruncate {
		return errInjectedTruncate
	}
	return f.journalFile.Truncate(size)
}

func (f *faultJournalFile) Sync() error {
	if f.failSync {
		f.failSync = false
		return errInjectedSync
	}
	return f.journalFile.Sync()
}

func TestAppendRollbackAfterHeaderAllowsLaterAppendAndReplay(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}

	fault := &faultJournalFile{journalFile: store.file, failPayloadWrite: true}
	store.file = fault
	if err := store.SaveBlock(genesis); !errors.Is(err, errInjectedPayloadWrite) {
		t.Fatalf("SaveBlock error=%v, want injected payload failure", err)
	}
	info, err := fault.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("journal size after successful rollback=%d, want 0", info.Size())
	}

	fault.failPayloadWrite = false
	fault.writes = 0
	if err := store.SaveBlock(genesis); err != nil {
		t.Fatalf("later SaveBlock after rollback: %v", err)
	}
	if err := store.SetTip(genesis.Hash); err != nil {
		t.Fatalf("SetTip after rollback: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	replayed, err := Open(dir)
	if err != nil {
		t.Fatalf("full replay after later append: %v", err)
	}
	defer replayed.Close()
	if !replayed.HasBlock(genesis.Hash) || replayed.Tip() != genesis.Hash {
		t.Fatal("replay did not recover the later successful block and tip records")
	}
}

func TestAppendRollbackPreservesEarlierFsyncedRecord(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBlock(genesis); err != nil {
		t.Fatalf("persist initial durable record: %v", err)
	}
	before, err := store.file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	fault := &faultJournalFile{journalFile: store.file, failPayloadWrite: true}
	store.file = fault
	if err := store.SetTip(genesis.Hash); !errors.Is(err, errInjectedPayloadWrite) {
		t.Fatalf("SetTip error=%v, want injected payload failure", err)
	}
	after, err := fault.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() != before.Size() {
		t.Fatalf("rollback journal size=%d, want preserve prior fsynced boundary %d", after.Size(), before.Size())
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	replayed, err := Open(dir)
	if err != nil {
		t.Fatalf("replay after later append rollback: %v", err)
	}
	defer replayed.Close()
	if !replayed.HasBlock(genesis.Hash) {
		t.Fatal("rollback removed an earlier successfully fsynced block record")
	}
	if replayed.Tip() != "" {
		t.Fatalf("failed tip append unexpectedly replayed tip %q", replayed.Tip())
	}
}


func TestAppendRollbackSyncFailureCanRecoverOnLaterAppendAndReplay(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}

	fault := &faultJournalFile{
		journalFile:      store.file,
		failPayloadWrite: true,
		failSync:         true,
	}
	store.file = fault
	err = store.SaveBlock(genesis)
	if !errors.Is(err, errInjectedPayloadWrite) || !errors.Is(err, errInjectedSync) {
		t.Fatalf("SaveBlock error=%v, want append and rollback-sync failures", err)
	}

	// Truncate succeeded before the rollback sync failed. A later append
	// seeks to the actual end, writes a clean record, and a normal close
	// syncs the recovered journal before replay.
	fault.failPayloadWrite = false
	fault.writes = 0
	if err := store.SaveBlock(genesis); err != nil {
		t.Fatalf("later append after rollback-sync failure: %v", err)
	}
	if err := store.SetTip(genesis.Hash); err != nil {
		t.Fatalf("SetTip after rollback-sync failure: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	replayed, err := Open(dir)
	if err != nil {
		t.Fatalf("replay after rollback-sync failure: %v", err)
	}
	defer replayed.Close()
	if !replayed.HasBlock(genesis.Hash) || replayed.Tip() != genesis.Hash {
		t.Fatal("replay did not recover later records after rollback-sync failure")
	}
}

func TestAppendRollbackFailureIsReportedAndReplayRejectsCorruptTail(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBlock(genesis); err != nil {
		t.Fatal(err)
	}

	fault := &faultJournalFile{
		journalFile:      store.file,
		failPayloadWrite: true,
		failTruncate:     true,
	}
	store.file = fault
	err = store.SetTip(genesis.Hash)
	if !errors.Is(err, errInjectedPayloadWrite) || !errors.Is(err, errInjectedTruncate) {
		t.Fatalf("SetTip error=%v, want append and rollback failures", err)
	}
	if !strings.Contains(err.Error(), "rollback incomplete journal record") {
		t.Fatalf("rollback failure not surfaced in error: %v", err)
	}

	// Simulate a subsequent append after rollback could not remove the partial
	// header. Replay must reject the malformed middle record rather than
	// silently skipping it or losing the earlier durable block.
	fault.failPayloadWrite = false
	fault.failTruncate = false
	fault.writes = 0
	if err := store.SaveStateCheckpoint(genesis.Hash, []byte("state")); err != nil {
		t.Fatalf("subsequent append: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if recovered, err := Open(dir); err == nil {
		_ = recovered.Close()
		t.Fatal("replay silently accepted a journal with an unremoved partial record")
	}
}
