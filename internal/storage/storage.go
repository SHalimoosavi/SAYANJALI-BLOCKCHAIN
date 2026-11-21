package storage

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/block"
	"github.com/SHalimoosavi/SAYANJALI-BLOCKCHAIN/internal/codec"
)

const (
	magic                        = "SYJDB001"
	version               uint16 = 1
	recordBlock           byte   = 1
	recordTip             byte   = 2
	recordStateCheckpoint byte   = 3
	maxRecord                    = 4 * 1024 * 1024
)

type journalFile interface {
	io.Reader
	io.Writer
	io.Seeker
	io.Closer
	Stat() (os.FileInfo, error)
	Truncate(int64) error
	Sync() error
}

type Store struct {
	mu          sync.Mutex
	dir         string
	file        journalFile
	blocks      map[string][]byte
	blockOrder  []string
	tip         string
	checkpoints map[string][]byte
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "ledger.journal"), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600) // #nosec G304 -- dir is the operator-configured node data directory; filename is fixed.
	if err != nil {
		return nil, err
	}
	s := &Store{dir: dir, file: f, blocks: make(map[string][]byte), checkpoints: make(map[string][]byte)}
	if err := s.replay(); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Sync()
	ce := s.file.Close()
	s.file = nil
	if err != nil {
		return err
	}
	return ce
}
func (s *Store) replay() error {
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	info, err := s.file.Stat()
	if err != nil {
		return err
	}
	fileSize := info.Size()
	r := bufio.NewReader(s.file)
	offset := int64(0)
	for {
		hdr := make([]byte, 8+2+1+4+4)
		n, err := io.ReadFull(r, hdr)
		if err == io.EOF && n == 0 {
			break
		}
		if err != nil {
			// A process crash can leave only a suffix of the final header on
			// disk. It is safe to discard that incomplete tail because no
			// complete record follows it. Genuine corruption in a complete
			// record remains fatal below.
			if offset+int64(n) == fileSize {
				return s.truncateTail(offset)
			}
			return fmt.Errorf("database corruption at offset %d: incomplete record header", offset)
		}
		if string(hdr[:8]) != magic {
			return fmt.Errorf("database corruption at offset %d: bad magic", offset)
		}
		if binary.BigEndian.Uint16(hdr[8:10]) != version {
			return fmt.Errorf("unsupported database version at offset %d", offset)
		}
		typ := hdr[10]
		length := binary.BigEndian.Uint32(hdr[11:15])
		wantCRC := binary.BigEndian.Uint32(hdr[15:19])
		if length > maxRecord {
			return fmt.Errorf("database corruption at offset %d: oversized record", offset)
		}
		payload := make([]byte, length)
		n, err = io.ReadFull(r, payload)
		if err != nil {
			if offset+int64(len(hdr))+int64(n) == fileSize {
				return s.truncateTail(offset)
			}
			return fmt.Errorf("database corruption at offset %d: incomplete payload", offset)
		}
		if crc32.ChecksumIEEE(payload) != wantCRC {
			return fmt.Errorf("database corruption at offset %d: checksum mismatch", offset)
		}
		switch typ {
		case recordBlock:
			b, err := codec.DecodeBlock(payload)
			if err != nil {
				return fmt.Errorf("database corruption at offset %d: %w", offset, err)
			}
			if b.Hash == "" {
				return fmt.Errorf("database corruption at offset %d: empty block hash", offset)
			}
			if _, exists := s.blocks[b.Hash]; !exists {
				s.blockOrder = append(s.blockOrder, b.Hash)
			}
			s.blocks[b.Hash] = append([]byte(nil), payload...)
		case recordTip:
			s.tip = string(payload)
		case recordStateCheckpoint:
			if len(payload) < 65 {
				return fmt.Errorf("database corruption at offset %d: invalid state checkpoint", offset)
			}
			hash := string(payload[:64])
			s.checkpoints[hash] = append([]byte(nil), payload[64:]...)
		default:
			return fmt.Errorf("database corruption at offset %d: unknown record type %d", offset, typ)
		}
		offset += int64(len(hdr)) + int64(length)
	}
	_, err = s.file.Seek(0, io.SeekEnd)
	return err
}
func (s *Store) truncateTail(offset int64) error {
	if err := s.file.Truncate(offset); err != nil {
		return fmt.Errorf("truncate incomplete journal tail at offset %d: %w", offset, err)
	}
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("sync recovered journal tail: %w", err)
	}
	if _, err := s.file.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	return nil
}

func (s *Store) appendRecord(typ byte, payload []byte) error {
	if len(payload) > maxRecord || uint64(len(payload)) > uint64(^uint32(0)) {
		return errors.New("record too large")
	}
	start, err := s.file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	rollback := func(cause error) error {
		if truncateErr := s.file.Truncate(start); truncateErr != nil {
			return errors.Join(cause, fmt.Errorf("rollback incomplete journal record: %w", truncateErr))
		}
		if syncErr := s.file.Sync(); syncErr != nil {
			return errors.Join(cause, fmt.Errorf("sync journal rollback: %w", syncErr))
		}
		if _, seekErr := s.file.Seek(0, io.SeekEnd); seekErr != nil {
			return errors.Join(cause, fmt.Errorf("seek after journal rollback: %w", seekErr))
		}
		return cause
	}
	var hdr [19]byte
	copy(hdr[:8], magic)
	binary.BigEndian.PutUint16(hdr[8:10], version)
	hdr[10] = typ
	binary.BigEndian.PutUint32(hdr[11:15], uint32(len(payload))) // #nosec G115 -- payload length is explicitly bounded to uint32.
	binary.BigEndian.PutUint32(hdr[15:19], crc32.ChecksumIEEE(payload))
	if n, writeErr := s.file.Write(hdr[:]); writeErr != nil {
		return rollback(writeErr)
	} else if n != len(hdr) {
		return rollback(io.ErrShortWrite)
	}
	if n, writeErr := s.file.Write(payload); writeErr != nil {
		return rollback(writeErr)
	} else if n != len(payload) {
		return rollback(io.ErrShortWrite)
	}
	if err := s.file.Sync(); err != nil {
		return rollback(err)
	}
	return nil
}
func (s *Store) SaveBlock(b *block.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, err := codec.BlockBytes(b)
	if err != nil {
		return err
	}
	if err = s.appendRecord(recordBlock, payload); err != nil {
		return err
	}
	if _, exists := s.blocks[b.Hash]; !exists {
		s.blockOrder = append(s.blockOrder, b.Hash)
	}
	s.blocks[b.Hash] = append([]byte(nil), payload...)
	return nil
}
func (s *Store) SetTip(hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if hash != "" {
		if _, ok := s.blocks[hash]; !ok {
			return errors.New("tip block is not stored")
		}
	}
	if err := s.appendRecord(recordTip, []byte(hash)); err != nil {
		return err
	}
	s.tip = hash
	return nil
}
func (s *Store) Tip() string { s.mu.Lock(); defer s.mu.Unlock(); return s.tip }
func (s *Store) GetBlock(hash string) (*block.Block, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.blocks[hash]
	if !ok {
		return nil, os.ErrNotExist
	}
	return codec.DecodeBlock(data)
}
func (s *Store) AllBlocks() ([]*block.Block, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*block.Block, 0, len(s.blocks))
	for _, hash := range s.blockOrder {
		data, ok := s.blocks[hash]
		if !ok {
			continue
		}
		b, err := codec.DecodeBlock(data)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}
func (s *Store) HasBlock(hash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.blocks[hash]
	return ok
}
func (s *Store) Path() string { return filepath.Join(s.dir, "ledger.journal") }
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	return s.file.Sync()
}

func (s *Store) SaveStateCheckpoint(blockHash string, payload []byte) error {
	if len(blockHash) != 64 {
		return errors.New("state checkpoint requires a 64-character block hash")
	}
	if len(payload) > maxRecord-64 {
		return errors.New("state checkpoint payload too large")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record := make([]byte, 64+len(payload))
	copy(record[:64], blockHash)
	copy(record[64:], payload)
	if err := s.appendRecord(recordStateCheckpoint, record); err != nil {
		return err
	}
	s.checkpoints[blockHash] = append([]byte(nil), payload...)
	return nil
}

func (s *Store) GetStateCheckpoint(blockHash string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.checkpoints[blockHash]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), p...), true
}
