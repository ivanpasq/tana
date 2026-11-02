package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const (
	setOperationKey = "set"
	delOperationKey = "del"
)

type Store struct {
	log     *os.File
	storage map[string]string
}

type op struct {
	Type  string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

func Open(path string) (*Store, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	store := Store{
		log:     f,
		storage: make(map[string]string),
	}

	if err := store.replay(); err != nil {
		f.Close()
		return nil, err
	}

	return &store, nil
}

func (s *Store) replay() error {
	r := bufio.NewReader(s.log)
	var bytesRead int
	for {
		var logEntry op

		b, err := r.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return err
		}
		if len(b) == 0 {
			break
		}

		finishWithNewLine := err == nil

		if err := json.Unmarshal(b, &logEntry); err != nil {
			if finishWithNewLine {
				return fmt.Errorf("corrupted log at byte %d: %w", bytesRead, err)
			}

			if err := s.log.Truncate(int64(bytesRead)); err != nil {
				return err
			}
			break
		}

		if err := s.apply(logEntry); err != nil {
			return err
		}
		bytesRead += len(b)

		if !finishWithNewLine {
			if _, err := s.log.Write([]byte("\n")); err != nil {
				return err
			}
			if err := s.log.Sync(); err != nil {
				return err
			}
			break
		}
	}

	return nil
}

func (s *Store) apply(logOp op) error {
	switch logOp.Type {
	case setOperationKey:
		s.applySet(logOp.Key, logOp.Value)
	case delOperationKey:
		s.applyDel(logOp.Key)
	default:
		return fmt.Errorf("unknown command found in log db: %v", logOp.Type)
	}

	return nil
}

func (s *Store) writeToLog(opType string, key string, value string) error {
	logEntry := op{
		Type:  opType,
		Key:   key,
		Value: value,
	}
	b, err := json.Marshal(logEntry)
	if err != nil {
		return err
	}
	b = append(b, "\n"...)

	if _, err := s.log.Write(b); err != nil {
		return err
	}

	return s.log.Sync()
}

func (s *Store) Close() error {
	return s.log.Close()
}

func (s *Store) Get(key string) (string, bool) {
	val, ok := s.storage[key]
	return val, ok
}

func (s *Store) Set(key string, value string) error {
	if err := s.writeToLog(setOperationKey, key, value); err != nil {
		return err
	}

	s.applySet(key, value)

	return nil
}

func (s *Store) applySet(key string, value string) {
	s.storage[key] = value
}

func (s *Store) Del(key string) (bool, error) {
	_, ok := s.storage[key]
	if !ok {
		return false, nil
	}

	if err := s.writeToLog(delOperationKey, key, ""); err != nil {
		return false, err
	}

	s.applyDel(key)
	return true, nil
}

func (s *Store) applyDel(key string) {
	delete(s.storage, key)
}
