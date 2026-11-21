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
	errInjectedSeek         = errors.New("injected seek failure")
)

type faultJournalFile struct {
	journalFile
	failPayloadWrite bool
	failTruncate     bool
	failSync         bool
	failSeek         bool
	skipSync         bool
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

func (f *faultJournalFile) Seek(offset int64, whence int) (int64, error) {
	if f.failSeek && f.writes > 0 {
		f.failSeek = false
		return 0, errInjectedSeek
	}
	return f.journalFile.Seek(offset, whence)
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
	if f.skipSync {
		return nil
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

	fault := &faultJournalFile{journalFile: store.file, failPayloadWrite: true, skipSync: true}
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
	replayed.file = unsyncedJournalFile{journalFile: replayed.file}
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
	replayed.file = unsyncedJournalFile{journalFile: replayed.file}
	defer replayed.Close()
	if !replayed.HasBlock(genesis.Hash) {
		t.Fatal("rollback removed an earlier successfully fsynced block record")
	}
	if replayed.Tip() != "" {
		t.Fatalf("failed tip append unexpectedly replayed tip %q", replayed.Tip())
	}
}

func TestAppendRollbackSyncFailurePoisonsStoreUntilReopen(t *testing.T) {
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
		failSync:         true,
		skipSync:         true,
	}
	fault.writes = 0
	store.file = fault
	err = store.SetTip(genesis.Hash)
	if !errors.Is(err, errInjectedPayloadWrite) || !errors.Is(err, errInjectedSync) {
		t.Fatalf("SetTip error=%v, want append and rollback-sync failures", err)
	}

	// Truncate succeeded, but its durability sync failed. The live Store
	// must reject every later append until it is closed and reopened.
	fault.failPayloadWrite = false
	fault.failSync = false
	fault.writes = 0
	if err := store.SaveBlock(genesis); err == nil || !strings.Contains(err.Error(), "store is failed after journal rollback failure") {
		t.Fatalf("later SaveBlock error=%v, want poisoned-store error", err)
	}
	if err := store.SetTip(genesis.Hash); err == nil || !strings.Contains(err.Error(), "store is failed after journal rollback failure") {
		t.Fatalf("later SetTip error=%v, want poisoned-store error", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen should replay the truncatable tail: %v", err)
	}
	defer reopened.Close()
	if !reopened.HasBlock(genesis.Hash) || reopened.Tip() != "" {
		t.Fatal("reopen did not preserve the prior state and discard the failed append")
	}
}

func TestAppendRollbackTruncateFailureIsReportedAndFinalTailRecovers(t *testing.T) {
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
		skipSync:         true,
	}
	store.file = fault
	err = store.SetTip(genesis.Hash)
	if !errors.Is(err, errInjectedPayloadWrite) || !errors.Is(err, errInjectedTruncate) {
		t.Fatalf("SetTip error=%v, want append and rollback failures", err)
	}
	if !strings.Contains(err.Error(), "rollback incomplete journal record") {
		t.Fatalf("rollback failure not surfaced in error: %v", err)
	}

	// The failed truncate leaves an incomplete final record. With no later
	// append placed behind it, startup recovery must discard only that tail
	// and retain the earlier successfully fsynced block.
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(dir)
	if err != nil {
		t.Fatalf("replay should recover the incomplete final tail: %v", err)
	}
	recovered.file = unsyncedJournalFile{journalFile: recovered.file}
	defer recovered.Close()
	if !recovered.HasBlock(genesis.Hash) {
		t.Fatal("replay dropped the earlier successfully fsynced block")
	}
	if recovered.Tip() != "" {
		t.Fatalf("failed tip append unexpectedly replayed tip %q", recovered.Tip())
	}
}

func TestAppendAfterRollbackTruncateFailureIsRejectedOnReplay(t *testing.T) {
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

	// Leave an incomplete SetTip record by making its payload write and
	// rollback truncate fail. The second append must now be rejected by
	// the poisoned Store, rather than being placed behind the corrupt tail.
	fault := &faultJournalFile{
		journalFile:      store.file,
		failPayloadWrite: true,
		failTruncate:     true,
		skipSync:         true,
	}
	store.file = fault
	err = store.SetTip(genesis.Hash)
	if !errors.Is(err, errInjectedPayloadWrite) || !errors.Is(err, errInjectedTruncate) {
		t.Fatalf("SetTip error=%v, want append and rollback failures", err)
	}

	fault.failPayloadWrite = false
	fault.failTruncate = false
	fault.writes = 0
	if err := store.SaveBlock(genesis); err == nil || !strings.Contains(err.Error(), "store is failed after journal rollback failure") {
		t.Fatalf("second append error=%v, want poisoned-store error", err)
	}
	if err := store.SetTip(genesis.Hash); err == nil || !strings.Contains(err.Error(), "store is failed after journal rollback failure") {
		t.Fatalf("later SetTip error=%v, want poisoned-store error", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen should recover the incomplete final tail: %v", err)
	}
	defer reopened.Close()
	if !reopened.HasBlock(genesis.Hash) || reopened.Tip() != "" {
		t.Fatal("reopen did not preserve the earlier block and discard the failed tip record")
	}
}

func TestAppendRollbackSeekFailurePoisonsStoreUntilReopen(t *testing.T) {
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
		failSeek:         true,
		skipSync:         true,
	}
	fault.writes = 0
	store.file = fault
	err = store.SetTip(genesis.Hash)
	if !errors.Is(err, errInjectedPayloadWrite) || !errors.Is(err, errInjectedSeek) {
		t.Fatalf("SetTip error=%v, want append and rollback-seek failures", err)
	}

	fault.failPayloadWrite = false
	fault.failSeek = false
	fault.writes = 0
	if err := store.SaveBlock(genesis); err == nil || !strings.Contains(err.Error(), "store is failed after journal rollback failure") {
		t.Fatalf("later SaveBlock error=%v, want poisoned-store error", err)
	}
	if err := store.SetTip(genesis.Hash); err == nil || !strings.Contains(err.Error(), "store is failed after journal rollback failure") {
		t.Fatalf("later SetTip error=%v, want poisoned-store error", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen should recover the truncated tail: %v", err)
	}
	defer reopened.Close()
	if !reopened.HasBlock(genesis.Hash) || reopened.Tip() != "" {
		t.Fatal("reopen did not preserve the earlier block and discard the failed tip record")
	}
}

func TestAppendSyncFailurePoisonsStoreUntilReopen(t *testing.T) {
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

	// The append itself is complete, but its first Sync fails. The existing
	// rollback truncates the attempted record and its following Sync succeeds.
	fault := &faultJournalFile{
		journalFile: store.file,
		failSync:    true,
	}
	store.file = fault
	if err := store.SaveBlock(genesis); !errors.Is(err, errInjectedSync) {
		t.Fatalf("SaveBlock error=%v, want injected append Sync failure", err)
	}

	// A later successful Sync cannot establish whether the failed fsync's
	// dirty pages were already lost. The live Store must stay poisoned.
	if err := store.SaveBlock(genesis); err == nil || !strings.Contains(err.Error(), "store is failed after journal rollback failure") {
		t.Fatalf("later SaveBlock error=%v, want failed-store error", err)
	}
	if err := store.SetTip(genesis.Hash); err == nil || !strings.Contains(err.Error(), "store is failed after journal rollback failure") {
		t.Fatalf("later SetTip error=%v, want failed-store error", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen after successful rollback should replay clean journal: %v", err)
	}
	defer reopened.Close()
	if !reopened.HasBlock(genesis.Hash) || reopened.Tip() != "" {
		t.Fatal("reopen did not preserve the prior durable block and clean tip state")
	}
}
