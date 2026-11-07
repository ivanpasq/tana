package store

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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

func TestReplayAndRepair(t *testing.T) {
	testCases := []struct {
		name               string
		logContent         string
		wantErr            bool
		wantStoreAfterOpen map[string]string
	}{
		{
			name:               "empty file",
			logContent:         "",
			wantStoreAfterOpen: map[string]string{},
		},
		{
			name:               "two valid lines",
			logContent:         `{"op":"set","key":"a","value":"1"}` + "\n" + `{"op":"set","key":"b","value":"2"}` + "\n",
			wantStoreAfterOpen: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:               "valid line + incomplete final line",
			logContent:         `{"op":"set","key":"a","value":"1"}` + "\n" + `{"op":"set","key":"b","va`,
			wantStoreAfterOpen: map[string]string{"a": "1"},
		},
		{
			name:       "valid + garbage with new line + valid",
			logContent: `{"op":"set","key":"a","value":"1"}` + "\n" + "garbage" + "\n" + `{"op":"set","key":"b","value":"2"}` + "\n",
			wantErr:    true,
		},
		{
			name:       "valid + garbage with new line at the end",
			logContent: `{"op":"set","key":"a","value":"1"}` + "\n" + "garbage" + "\n",
			wantErr:    true,
		},
		{
			name:               "valid + valid without new line at the end",
			logContent:         `{"op":"set","key":"a","value":"1"}` + "\n" + `{"op":"set","key":"b","value":"2"}`,
			wantStoreAfterOpen: map[string]string{"a": "1", "b": "2"},
		},
		{
			name:       "valid + unknown command",
			logContent: `{"op":"set","key":"a","value":"1"}` + "\n" + `{"op":"boh","key":"x"}` + "\n",
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "test.db")
			err := os.WriteFile(logPath, []byte(tc.logContent), 0o600)
			if err != nil {
				t.Fatalf("error while creating log file: %v", err)
			}

			s, err := Open(logPath)
			if tc.wantErr != (err != nil) {
				t.Errorf("error, got %v, want error %v", err, tc.wantErr)
				return
			}
			if tc.wantErr {
				CheckCorruptedFileNotUpdated(t, logPath, tc.logContent)
				return
			}

			if !maps.Equal(s.storage, tc.wantStoreAfterOpen) {
				t.Errorf("map after first open, got %v, want %v", s.storage, tc.wantStoreAfterOpen)
				return
			}

			if err := s.Set("z", "9"); err != nil {
				t.Fatalf("error setting z key: %v", err)
			}
			tcMaps := maps.Clone(tc.wantStoreAfterOpen)
			tcMaps["z"] = "9"

			if err := s.Close(); err != nil {
				t.Fatalf("error closing store: %v", err)
			}

			s, err = Open(logPath)
			if err != nil {
				t.Fatalf("error restarting store: %v", err)
			}
			defer s.Close()

			if !maps.Equal(s.storage, tcMaps) {
				t.Errorf("store maps after reopening, got %v, want %v", s.storage, tcMaps)
			}
		})
	}
}

func CheckCorruptedFileNotUpdated(t *testing.T, logPath string, expectedContent string) {
	t.Helper()

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("error while reading log file: %v", err)
	}

	if string(log) != expectedContent {
		t.Errorf("corrupted file, got %s, want %s", string(log), expectedContent)
	}
}

func TestRaceCondition(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := OpenStore(t, dbPath)
	defer s.Close()

	var wg sync.WaitGroup

	for i := range 100 {
		wg.Go(func() {
			k := fmt.Sprintf("test_key_%d", i)
			v := fmt.Sprintf("test_value_%d", i)

			if err := s.Set(k, v); err != nil {
				t.Errorf("set, error: %v", err)
				return
			}

			result, ok := s.Get(k)
			if !ok {
				t.Errorf("get, failed")
				return
			}
			if result != v {
				t.Errorf("get, got %s, want %s", result, v)
				return
			}

			ok, err := s.Del(k)
			if err != nil {
				t.Errorf("del, got error: %v", err)
				return
			}
			if !ok {
				t.Errorf("del, failed")
				return
			}
		})
	}

	wg.Wait()

	if len(s.storage) != 0 {
		t.Errorf("len storage, got %d, want 0", len(s.storage))
	}
}

func TestRaceConditionOnSameKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s := OpenStore(t, dbPath)
	defer s.Close()

	if err := s.Set("key", "value"); err != nil {
		t.Fatalf("set key, error: %v", err)
	}

	var wg sync.WaitGroup
	var n atomic.Int32

	for range 50 {
		wg.Go(func() {
			ok, err := s.Del("key")
			if err != nil {
				t.Errorf("del error: %v", err)
				return
			}
			if ok {
				n.Add(1)
			}
		})
	}

	wg.Wait()

	if n.Load() != 1 {
		t.Errorf("ok deletions, got %d, want 1", n.Load())
	}
}
