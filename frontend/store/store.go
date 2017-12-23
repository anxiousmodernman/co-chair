package store

import (
	"log"
	"strings"
	"sync"
)

var S *Store

func init() {
	S = NewStore()
}

// Store is a state container for our frontend app.
// TODO remove mutex?
type Store struct {
	mtx         *sync.RWMutex
	data        map[string]interface{}
	subscribers map[string]map[int]func()
	counter     int
}

// NewStore is our store constructor.
func NewStore() *Store {
	var s Store
	s.mtx = &sync.RWMutex{}
	s.data = make(map[string]interface{})
	s.subscribers = make(map[string]map[int]func())
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

// Subscribe will call a callback on new puts to key. Returned
// is the callback ID. Callers can store this and use it later to
// de-register their callbacks.
func (s *Store) Subscribe(key string, cb func()) int {
	s.mtx.Lock()
	s.counter++
	defer s.mtx.Unlock()
	if m, ok := s.subscribers[key]; ok {
		// there are other subscribers to this key
		m[s.counter] = cb
		s.subscribers[key] = m
	} else {
		m := make(map[int]func())
		m[s.counter] = cb
		s.subscribers[key] = m
	}
	log.Println("subscribed:", s.counter)
	return s.counter
}

// Unsubscribe removes the callback associated with the given id.
func (s *Store) Unsubscribe(callbackID int) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for _, m := range s.subscribers {
		for k := range m {
			if k == callbackID {
				// wacky golang delete syntax
				delete(m, k)
			}
		}
	}
}

func (s *Store) notify(key string) {
	log.Println("notify!")
	for k, vals := range s.subscribers {
		if strings.HasPrefix(k, key) {
			for _, fn := range vals {
				fn()
			}
		}
	}
}
