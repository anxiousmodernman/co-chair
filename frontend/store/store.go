package store

import (
	"strings"
	"sync"
)

// Store is a state container for our frontend app.
type Store struct {
	mtx         *sync.RWMutex
	data        map[string]interface{}
	subscribers map[string][]func()
}

// NewStore is our store constructor.
func NewStore() *Store {
	var s Store
	s.mtx = &sync.RWMutex{}
	s.data = make(map[string]interface{})
	s.subscribers = make(map[string][]func())
	return &s
}

// Put adds data to our store.
func (s *Store) Put(key string, val interface{}) {
	if key == "" {
		return
	}
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.data[key] = val
	s.notify(key)
}

// Get returns data associated with key, or nil.
func (s *Store) Get(key string) interface{} {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.data[key]
}

// Subscribe will call a callback on new puts to key.
func (s *Store) Subscribe(key string, cb func()) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if m, ok := s.subscribers[key]; ok {
		m = append(m, cb)
		s.subscribers[key] = m
	} else {
		callbacks := []func(){cb}
		m := make(map[string][]func())
		m[key] = callbacks
	}
}

func (s *Store) notify(key string) {
	for k, vals := range s.subscribers {
		if strings.HasPrefix(k, key) {
			for _, fn := range vals {
				fn()
			}
		}
	}
}
