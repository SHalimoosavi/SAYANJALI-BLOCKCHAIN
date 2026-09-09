package storage

import (
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
)

func TestJournalPersistsAndRecovers(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	g, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveBlock(g); err != nil {
		t.Fatal(err)
	}
	if err = s.SetTip(g.Hash); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.GetBlock(g.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hash != g.Hash || s.Tip() != g.Hash {
		t.Fatal("persisted state mismatch")
	}
}
func TestJournalCorruptionDetected(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := block.Genesis()
	if err = s.SaveBlock(g); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(s.Path(), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt a byte inside the complete genesis record payload.
	if _, err := f.Seek(19, 0); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := f.Read(one[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(19, 0); err != nil {
		t.Fatal(err)
	}
	one[0] ^= 0xff
	if _, err := f.Write(one[:]); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if _, err = Open(dir); err == nil {
		t.Fatal("expected corruption error")
	}
}

func TestJournalRecoversTruncatedFinalHeader(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := block.Genesis()
	if err := s.SaveBlock(g); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "ledger.journal"), os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte("partial"))
	_ = f.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !s.HasBlock(g.Hash) || s.Tip() != "" {
		t.Fatal("valid state not preserved after header-tail recovery")
	}
}

func TestJournalRecoversTruncatedFinalPayload(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := block.Genesis()
	if err := s.SaveBlock(g); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTip(g.Hash); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// A valid record header declaring a 64-byte payload, followed by only a
	// partial payload, models a crash during the final write.
	var hdr [19]byte
	copy(hdr[:8], magic)
	binary.BigEndian.PutUint16(hdr[8:10], version)
	hdr[10] = recordTip
	binary.BigEndian.PutUint32(hdr[11:15], 64)
	binary.BigEndian.PutUint32(hdr[15:19], crc32.ChecksumIEEE(make([]byte, 64)))
	f, err := os.OpenFile(filepath.Join(dir, "ledger.journal"), os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write(hdr[:])
	_, _ = f.Write([]byte("short"))
	_ = f.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !s.HasBlock(g.Hash) || s.Tip() != g.Hash {
		t.Fatal("valid state not preserved after payload-tail recovery")
	}
}

func TestJournalCRCcorruptionRemainsFatal(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := block.Genesis()
	if err := s.SaveBlock(g); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "ledger.journal"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(19, 0); err != nil {
		t.Fatal(err)
	}
	var one [1]byte
	if _, err := f.Read(one[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(19, 0); err != nil {
		t.Fatal(err)
	}
	one[0] ^= 0xff
	if _, err := f.Write(one[:]); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if _, err := Open(dir); err == nil {
		t.Fatal("expected CRC corruption to remain fatal")
	}
}

func TestJournalMultipleRecordsRecoverDeterministically(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := block.Genesis()
	if err := s.SaveBlock(g); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTip(g.Hash); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "ledger.journal"), os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte("tail"))
	_ = f.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Tip() != g.Hash || !s.HasBlock(g.Hash) {
		t.Fatal("multiple valid records were not preserved")
	}
	_ = s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Tip() != g.Hash || !s.HasBlock(g.Hash) {
		t.Fatal("recovered journal is not deterministic across restart")
	}
}
