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
	magic              = "SYJDB001"
	version     uint16 = 1
	recordBlock byte   = 1
	recordTip   byte   = 2
	maxRecord          = 4 * 1024 * 1024
)

type Store struct {
	mu     sync.Mutex
	dir    string
	file   *os.File
	blocks map[string][]byte
	tip    string
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "ledger.journal"), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	s := &Store{dir: dir, file: f, blocks: make(map[string][]byte)}
	if err := s.replay(); err != nil {
		f.Close()
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
	r := bufio.NewReader(s.file)
	offset := int64(0)
	for {
		hdr := make([]byte, 8+2+1+4+4)
		n, err := io.ReadFull(r, hdr)
		if err == io.EOF && n == 0 {
			break
		}
		if err != nil {
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
		if _, err := io.ReadFull(r, payload); err != nil {
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
			s.blocks[b.Hash] = append([]byte(nil), payload...)
		case recordTip:
			s.tip = string(payload)
		default:
			return fmt.Errorf("database corruption at offset %d: unknown record type %d", offset, typ)
		}
		offset += int64(len(hdr)) + int64(length)
	}
	_, err := s.file.Seek(0, io.SeekEnd)
	return err
}
func (s *Store) appendRecord(typ byte, payload []byte) error {
	if len(payload) > maxRecord {
		return errors.New("record too large")
	}
	var hdr [19]byte
	copy(hdr[:8], magic)
	binary.BigEndian.PutUint16(hdr[8:10], version)
	hdr[10] = typ
	binary.BigEndian.PutUint32(hdr[11:15], uint32(len(payload)))
	binary.BigEndian.PutUint32(hdr[15:19], crc32.ChecksumIEEE(payload))
	if _, err := s.file.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := s.file.Write(payload); err != nil {
		return err
	}
	return s.file.Sync()
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
	for _, data := range s.blocks {
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
