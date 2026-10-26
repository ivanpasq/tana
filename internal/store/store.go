package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

const setOperationKey = "set"
const delOperationKey = "del"

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
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}

	store := Store{
		log:     f,
		storage: make(map[string]string),
	}

	if err := store.replay(); err != nil {
		return nil, err
	}

	return &store, nil
}

func (s *Store) replay() error {
	sc := bufio.NewScanner(s.log)
	for sc.Scan() {
		var logEntry op
		if err := json.Unmarshal([]byte(sc.Text()), &logEntry); err != nil {
			return err
		}

		switch logEntry.Type {
		case setOperationKey:
			s.applySet(logEntry.Key, logEntry.Value)
		case delOperationKey:
			s.applyDel(logEntry.Key)
		default:
			return fmt.Errorf("unknown command found in log db: %v", logEntry.Type)
		}
	}

	return sc.Err()
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
