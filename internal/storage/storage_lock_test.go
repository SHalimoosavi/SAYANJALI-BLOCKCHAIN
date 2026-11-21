package storage

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
)

const storageLockHelperEnv = "SYJ_STORAGE_OPEN_LOCK_HELPER_DIR"

func TestOpenSameDirectoryTwiceInProcessFailsClosed(t *testing.T) {
	dir := t.TempDir()
	holder, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()

	second, err := Open(dir)
	if err == nil {
		_ = second.Close()
		t.Fatal("second Open on the same data directory succeeded; want exclusive-lock error")
	}
	if !strings.Contains(err.Error(), "data directory is already locked") {
		t.Fatalf("second Open error=%q, want clear data-directory lock error", err)
	}
}

func TestOpenSameDirectoryFromSecondProcessFailsClosed(t *testing.T) {
	dir := t.TempDir()
	holder, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()

	cmd, stdout, stdin := startStorageOpenLockHelper(t, dir)
	line := readStorageOpenLockHelperResult(t, stdout)
	if strings.HasPrefix(line, "OPENED") {
		_, _ = fmt.Fprintln(stdin, "release")
		_ = cmd.Wait()
		t.Fatal("second process opened a data directory already held by this process")
	}
	if !strings.HasPrefix(line, "REJECTED:") || !strings.Contains(line, "data directory is already locked") {
		t.Fatalf("second-process result=%q, want lock rejection", line)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper process exit: %v", err)
	}
}

func TestOpenSucceedsAfterHolderCloses(t *testing.T) {
	dir := t.TempDir()
	holder, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("Open after holder Close: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenSucceedsAfterHolderProcessIsKilled(t *testing.T) {
	dir := t.TempDir()
	cmd, stdout, _ := startStorageOpenLockHelper(t, dir)
	line := readStorageOpenLockHelperResult(t, stdout)
	if line != "OPENED" {
		_ = cmd.Wait()
		t.Fatalf("holder process result=%q, want OPENED", line)
	}
	if err := cmd.Process.Kill(); err != nil {
		_ = cmd.Wait()
		t.Fatalf("kill holder process: %v", err)
	}
	_ = cmd.Wait()

	reopened, err := Open(dir)
	if err != nil {
		t.Fatalf("Open after holder process death: %v; lock must not become stale", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectedOpenDoesNotTouchJournalBytes(t *testing.T) {
	dir := t.TempDir()
	holder, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()

	genesis, err := block.Genesis()
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.SaveBlock(genesis); err != nil {
		t.Fatal(err)
	}

	// Leave a recoverable incomplete tail while the holder retains the lock.
	// An opener that reaches replay without first acquiring the directory lock
	// will truncate this tail, changing the journal bytes and size.
	journalPath := filepath.Join(dir, "ledger.journal")
	tail, err := os.OpenFile(journalPath, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tail.Write([]byte("partial")); err != nil {
		_ = tail.Close()
		t.Fatal(err)
	}
	if err := tail.Sync(); err != nil {
		_ = tail.Close()
		t.Fatal(err)
	}
	if err := tail.Close(); err != nil {
		t.Fatal(err)
	}

	before, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(journalPath)
	if err != nil {
		t.Fatal(err)
	}

	rejected, openErr := Open(dir)
	if openErr == nil {
		_ = rejected.Close()
		t.Fatal("Open succeeded while another Store held the directory lock")
	}
	if !strings.Contains(openErr.Error(), "data directory is already locked") {
		t.Fatalf("rejected Open error=%q, want directory-lock error", openErr)
	}

	after, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	afterInfo, err := os.Stat(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if beforeInfo.Size() != afterInfo.Size() || !bytes.Equal(before, after) {
		t.Fatalf("rejected Open changed journal: size %d -> %d, bytes_equal=%v",
			beforeInfo.Size(), afterInfo.Size(), bytes.Equal(before, after))
	}
}

func TestJournalFormatAndReplayRemainUnchanged(t *testing.T) {
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
	if err := store.SetTip(genesis.Hash); err != nil {
		t.Fatal(err)
	}
	journalPath := store.Path()
	before, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) < 19 || string(before[:8]) != magic {
		t.Fatalf("journal header does not retain existing magic %q", magic)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	replayed, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen existing journal: %v", err)
	}
	defer replayed.Close()
	if !replayed.HasBlock(genesis.Hash) || replayed.Tip() != genesis.Hash {
		t.Fatal("existing journal replay produced different block/tip state")
	}
	after, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("replay changed bytes of a complete existing journal")
	}
}

func TestStorageOpenLockHelperProcess(t *testing.T) {
	dir := os.Getenv(storageLockHelperEnv)
	if dir == "" {
		return
	}
	store, err := Open(dir)
	if err != nil {
		fmt.Fprintf(os.Stdout, "SYJ_STORAGE_LOCK_HELPER:REJECTED:%v\n", err)
		return
	}
	fmt.Fprintln(os.Stdout, "SYJ_STORAGE_LOCK_HELPER:OPENED")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	_ = store.Close()
}

func startStorageOpenLockHelper(t *testing.T, dir string) (*exec.Cmd, *bufio.Reader, io.WriteCloser) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.v", "-test.run=^TestStorageOpenLockHelperProcess$")
	cmd.Env = append(os.Environ(), storageLockHelperEnv+"="+dir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return cmd, bufio.NewReader(stdout), stdin
}

func readStorageOpenLockHelperResult(t *testing.T, stdout *bufio.Reader) string {
	t.Helper()
	for {
		line, err := stdout.ReadString('\n')
		if err != nil {
			t.Fatalf("read helper process result: %v", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SYJ_STORAGE_LOCK_HELPER:") {
			return strings.TrimPrefix(line, "SYJ_STORAGE_LOCK_HELPER:")
		}
	}
}
