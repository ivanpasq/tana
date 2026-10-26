package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen(t *testing.T) {
	t.Run("Open on not existant file", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s := OpenStore(t, dbPath)
		defer s.Close()

		if len(s.storage) != 0 {
			t.Errorf("store len: got %d, want %d", len(s.storage), 0)
		}

		_, err := os.Open(s.log.Name())
		if err != nil {
			t.Errorf("db file created, got error %v, want no errors", err)
		}
	})

	t.Run("closing and opening the store retains previous operations after restarting", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s := OpenStore(t, dbPath)

		if err := s.Set("testKey", "testValue"); err != nil {
			t.Fatalf("error setting value: %v", err)
		}

		if err := s.Close(); err != nil {
			t.Fatalf("error closing store: %v", err)
		}

		s = OpenStore(t, dbPath)
		defer s.Close()

		v, ok := s.Get("testKey")
		if !ok {
			t.Error("testKey not found")
			return
		}

		if v != "testValue" {
			t.Errorf("get testKey, got %v, want %v", v, "testValue")
		}
	})

	t.Run("setting and deleting retains operation after restarting", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s := OpenStore(t, dbPath)

		if err := s.Set("testKey", "testValue"); err != nil {
			t.Fatalf("error setting testKey: %v", err)
		}

		ok, err := s.Del("testKey")
		if !ok {
			t.Fatal("deleting testKey is not ok")
		}
		if err != nil {
			t.Fatalf("error deleting testKey: %v", err)
		}

		if err := s.Close(); err != nil {
			t.Fatalf("error closing store: %v", err)
		}

		s = OpenStore(t, dbPath)
		defer s.Close()

		v, ok := s.Get("testKey")
		if ok {
			t.Errorf("getting testKey, got value %v, expected empty", v)
		}
	})

	t.Run("setting two time same key retains second value after restarting", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s := OpenStore(t, dbPath)

		if err := s.Set("testKey", "firstValue"); err != nil {
			t.Fatalf("error setting testKey: %v", err)
		}

		if err := s.Set("testKey", "secondValue"); err != nil {
			t.Fatalf("error setting testKey: %v", err)
		}

		if err := s.Close(); err != nil {
			t.Fatalf("error closing store: %v", err)
		}

		s = OpenStore(t, dbPath)
		defer s.Close()

		v, ok := s.Get("testKey")
		if !ok {
			t.Error("getting testKey, no value found, expected secondValue")
			return
		}
		if v != "secondValue" {
			t.Errorf("getting testKey, got %v, want %v", v, "secondValue")
		}
	})

	t.Run("getting complex value after restarting", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s := OpenStore(t, dbPath)

		if err := s.Set("testKey", "'\t\tcomplex value with spaces\t'"); err != nil {
			t.Fatalf("error setting testKey: %v", err)
		}

		if err := s.Close(); err != nil {
			t.Fatalf("error closing store: %v", err)
		}

		s = OpenStore(t, dbPath)
		defer s.Close()

		v, ok := s.Get("testKey")
		if !ok {
			t.Errorf("getting testKey, got empty value, want some values")
			return
		}
		if v != "'\t\tcomplex value with spaces\t'" {
			t.Errorf("getting testKey, got %v, want %v", v, "'\t\tcomplex value with spaces\t'")
		}
	})

	t.Run("opening store does not affect its db log", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s := OpenStore(t, dbPath)

		if err := s.Set("testKey", "testValue"); err != nil {
			t.Fatalf("error setting testKey: %v", err)
		}

		dbFileInfo, err := s.log.Stat()
		if err != nil {
			t.Fatalf("error retriving db file info: %v", err)
		}
		dbFileSizeBeforeClosing := dbFileInfo.Size()

		if err := s.Close(); err != nil {
			t.Fatalf("error closing store: %v", err)
		}

		s = OpenStore(t, dbPath)
		defer s.Close()

		dbFileInfo, err = s.log.Stat()
		if err != nil {
			t.Fatalf("error retriving db file info after restarting: %v", err)
		}

		if dbFileInfo.Size() != dbFileSizeBeforeClosing {
			t.Errorf("comparing db file size after restart, got %d, want %d", dbFileInfo.Size(), dbFileSizeBeforeClosing)
		}
	})
}

func OpenStore(t *testing.T, dbPath string) *Store {
	t.Helper()

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("error opening store: %v", err)
	}

	return s
}
