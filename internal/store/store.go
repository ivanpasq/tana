package store

type Store struct {
	storage map[string]string
}

func New() *Store {
	return &Store{
		storage: make(map[string]string),
	}
}

func (s *Store) Get(key string) (string, bool) {
	val, ok := s.storage[key]
	return val, ok
}

func (s *Store) Set(key string, value string) {
	s.storage[key] = value
}

func (s *Store) Del(key string) bool {
	_, ok := s.storage[key]
	if !ok {
		return false
	}

	delete(s.storage, key)
	return true
}
