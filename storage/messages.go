package storage

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/dgraph-io/badger/v4"
)

const messagePrefix = "message:"
const maxMessages = 200

type MessageRecord struct {
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

type MessageStore struct {
	db *badger.DB
}

func OpenMessageStore(baseDir string) (*MessageStore, error) {
	dbPath := filepath.Join(baseDir, "messages")
	db, err := badger.Open(badger.DefaultOptions(dbPath))
	if err != nil {
		return nil, fmt.Errorf("open badger db: %w", err)
	}

	store := &MessageStore{db: db}
	if err := store.trimToLimit(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *MessageStore) Save(record MessageRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	key := []byte(messagePrefix + strconv.FormatInt(record.Timestamp.UnixNano(), 10))
	err = s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(key, payload); err != nil {
			return err
		}

		return trimToLimitTxn(txn)
	})
	if err != nil {
		return fmt.Errorf("save message: %w", err)
	}

	return nil
}

func (s *MessageStore) LoadAll() ([]MessageRecord, error) {
	records := make([]MessageRecord, 0)

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(messagePrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var record MessageRecord
				if err := json.Unmarshal(val, &record); err != nil {
					return fmt.Errorf("unmarshal message: %w", err)
				}

				records = append(records, record)
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load messages: %w", err)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	return records, nil
}

func (s *MessageStore) Clear() error {
	err := s.db.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(messagePrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			if err := txn.Delete(key); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("clear messages: %w", err)
	}

	return nil
}

func (s *MessageStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}

	return s.db.Close()
}

func (s *MessageStore) trimToLimit() error {
	err := s.db.Update(func(txn *badger.Txn) error {
		return trimToLimitTxn(txn)
	})
	if err != nil {
		return fmt.Errorf("trim messages: %w", err)
	}

	return nil
}

func trimToLimitTxn(txn *badger.Txn) error {
	keys := make([][]byte, 0, maxMessages+1)

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	prefix := []byte(messagePrefix)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		key := it.Item().KeyCopy(nil)
		keys = append(keys, key)
	}

	excess := len(keys) - maxMessages
	if excess <= 0 {
		return nil
	}

	for _, key := range keys[:excess] {
		if err := txn.Delete(key); err != nil {
			return err
		}
	}

	return nil
}
