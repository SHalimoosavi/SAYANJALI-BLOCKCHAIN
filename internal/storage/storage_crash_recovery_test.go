package storage

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
)

func TestJournalCrashRecoveryAtEveryByteBoundary(t *testing.T) {
	sourceDir := t.TempDir()
	source, err := Open(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}
	if err := source.SaveBlock(genesis); err != nil {
		t.Fatal(err)
	}
	checkpointPayload := []byte("deterministic-test-state")
	if err := source.SaveStateCheckpoint(genesis.Hash, checkpointPayload); err != nil {
		t.Fatal(err)
	}
	if err := source.SetTip(genesis.Hash); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	journal, err := os.ReadFile(filepath.Join(sourceDir, "ledger.journal"))
	if err != nil {
		t.Fatal(err)
	}

	var recordEnds []int
	for offset := 0; offset < len(journal); {
		if len(journal)-offset < 19 {
			t.Fatalf("source journal has an incomplete header at offset %d", offset)
		}
		payloadLength := int(binary.BigEndian.Uint32(journal[offset+11 : offset+15]))
		end := offset + 19 + payloadLength
		if end > len(journal) {
			t.Fatalf("source journal has an incomplete record at offset %d", offset)
		}
		recordEnds = append(recordEnds, end)
		offset = end
	}
	if len(recordEnds) != 3 {
		t.Fatalf("source journal contains %d records, want block/checkpoint/tip", len(recordEnds))
	}

	for cut := 0; cut <= len(journal); cut++ {
		t.Run(strconv.Itoa(cut), func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "ledger.journal"), journal[:cut], 0600); err != nil {
				t.Fatal(err)
			}

			recovered, err := Open(dir)
			if err != nil {
				t.Fatalf("Open after truncation at byte %d: %v", cut, err)
			}
			defer recovered.Close()

			completeRecords := 0
			lastCompleteEnd := 0
			for _, end := range recordEnds {
				if end <= cut {
					completeRecords++
					lastCompleteEnd = end
				}
			}

			if got, want := recovered.HasBlock(genesis.Hash), completeRecords >= 1; got != want {
				t.Fatalf("block present=%v, want %v after %d complete records", got, want, completeRecords)
			}
			checkpoint, checkpointOK := recovered.GetStateCheckpoint(genesis.Hash)
			wantCheckpoint := completeRecords >= 2
			if checkpointOK != wantCheckpoint {
				t.Fatalf("checkpoint present=%v, want %v after %d complete records", checkpointOK, wantCheckpoint, completeRecords)
			}
			if wantCheckpoint && string(checkpoint) != string(checkpointPayload) {
				t.Fatalf("checkpoint payload=%q, want %q", checkpoint, checkpointPayload)
			}
			wantTip := ""
			if completeRecords >= 3 {
				wantTip = genesis.Hash
			}
			if got := recovered.Tip(); got != wantTip {
				t.Fatalf("tip=%q, want %q after %d complete records", got, wantTip, completeRecords)
			}

			info, err := os.Stat(recovered.Path())
			if err != nil {
				t.Fatal(err)
			}
			if got, want := info.Size(), int64(lastCompleteEnd); got != want {
				t.Fatalf("recovered journal size=%d, want last complete record boundary %d", got, want)
			}
		})
	}
}
