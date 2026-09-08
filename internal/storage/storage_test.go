package storage

import (
	"os"
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
	f, err := os.OpenFile(s.Path(), os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte("corrupt"))
	_ = f.Close()
	if _, err = Open(dir); err == nil {
		t.Fatal("expected corruption error")
	}
}
