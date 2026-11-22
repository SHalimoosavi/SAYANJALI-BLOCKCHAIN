package chain

import (
	"errors"
	"os"
	"reflect"
	"testing"
	"unsafe"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/storage"
)

func TestAcceptOrdinaryRejectionsDoNotWrapSentinel(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ch, err := Open(st)
	if err != nil {
		t.Fatal(err)
	}
	ok, reason, err := ch.Accept(ch.Tip())
	if err != nil || ok || reason != "duplicate" {
		t.Fatalf("duplicate Accept = (%v, %q, %v), want false, duplicate, nil", ok, reason, err)
	}
	if errors.Is(err, storage.ErrStoreFailed) {
		t.Fatalf("ordinary duplicate rejection wraps storage sentinel: %v", err)
	}
}

// l02FailSyncFile injects one deterministic journal fsync failure while
// preserving the real file's remaining behavior. It is installed only in
// tests by replacing Store.file through reflection because that field and the
// journalFile interface are intentionally private to storage.
type l02FailSyncFile struct {
	*os.File
	failAt int
	syncs  int
}

func (f *l02FailSyncFile) Sync() error {
	f.syncs++
	if f.syncs == f.failAt {
		return errors.New("injected chain journal sync failure")
	}
	return f.File.Sync()
}

func l02InstallFailSync(t *testing.T, st *storage.Store, failAt int) {
	t.Helper()
	field := reflect.ValueOf(st).Elem().FieldByName("file")
	if !field.IsValid() || !field.CanAddr() {
		t.Fatal("storage Store.file field is unavailable")
	}
	file := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface().(*os.File)
	fault := &l02FailSyncFile{File: file, failAt: failAt}
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(fault))
}

func TestAcceptPreservesStoreFailedSentinel(t *testing.T) {
	for _, tc := range []struct {
		name       string
		failAtSync int
		prepareTo  int
	}{
		{name: "SaveBlock", failAtSync: 1, prepareTo: 0},
		{name: "SaveStateCheckpoint", failAtSync: 2, prepareTo: 15},
		{name: "SetTip", failAtSync: 3, prepareTo: 15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st, err := storage.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			ch, err := Open(st)
			if err != nil {
				t.Fatal(err)
			}
			for height := 1; height <= tc.prepareTo; height++ {
				parent := ch.Tip()
				b := mineTestBlock(t, parent, "SYJaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", parent.Timestamp+31)
				if ok, reason, acceptErr := ch.Accept(b); acceptErr != nil || !ok || reason != "best" {
					t.Fatalf("prepare height %d: Accept=(%v,%q,%v)", height, ok, reason, acceptErr)
				}
			}
			parent := ch.Tip()
			next := mineTestBlock(t, parent, "SYJaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", parent.Timestamp+31)
			l02InstallFailSync(t, st, tc.failAtSync)
			ok, reason, acceptErr := ch.Accept(next)
			if ok || reason != "storage" {
				t.Fatalf("Accept=(%v,%q,%v), want false, storage, error", ok, reason, acceptErr)
			}
			if !errors.Is(acceptErr, storage.ErrStoreFailed) {
				t.Fatalf("Accept error for %s = %v, want errors.Is(err, storage.ErrStoreFailed)", tc.name, acceptErr)
			}
		})
	}
}
