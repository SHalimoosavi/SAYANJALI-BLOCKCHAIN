package storage

import (
 "errors"
 "testing"
 "github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
)

func TestStoreFailedErrorsWrapSentinel(t *testing.T) {
 st,err:=Open(t.TempDir()); if err!=nil {t.Fatal(err)}; defer st.Close()
 g,err:=block.Genesis(); if err!=nil {t.Fatal(err)}
 st.file=&faultJournalFile{journalFile:st.file,failSync:true}
 err=st.SaveBlock(g)
 if !errors.Is(err,ErrStoreFailed) {t.Fatalf("SaveBlock error = %v, want errors.Is(err, ErrStoreFailed)",err)}
}
func TestSyncFailureAlwaysSatisfiesSentinel(t *testing.T) {
 st,err:=Open(t.TempDir()); if err!=nil {t.Fatal(err)}; defer st.Close()
 g,err:=block.Genesis(); if err!=nil {t.Fatal(err)}
 st.file=&faultJournalFile{journalFile:st.file,failSync:true}
 err=st.SaveBlock(g)
 if !errors.Is(err,ErrStoreFailed) {t.Fatalf("journal Sync failure error = %v, want errors.Is(err, ErrStoreFailed), including successful rollback",err)}
}
func TestOrdinaryWriteErrorDoesNotWrapSentinel(t *testing.T) {
 st,err:=Open(t.TempDir()); if err!=nil {t.Fatal(err)}; defer st.Close()
 g,err:=block.Genesis(); if err!=nil {t.Fatal(err)}
 f:=&faultJournalFile{journalFile:st.file,failPayloadWrite:true,skipSync:true}; st.file=f
 err=st.SaveBlock(g); if err==nil {t.Fatal("expected injected ordinary write error")}
 if errors.Is(err,ErrStoreFailed) {t.Fatalf("ordinary write error unexpectedly wraps ErrStoreFailed: %v",err)}
 f.failPayloadWrite=false; f.writes=0
 if err:=st.SaveBlock(g); err!=nil {t.Fatalf("store unusable after successful rollback: %v",err)}
}
