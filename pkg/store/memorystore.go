// Package store implements a simple key-value store.
package store

import (
	"errors"
	"log"
	"sync"
)

var (
	ErrKeyExists      = errors.New("store: key already exists")
	ErrKeyDoesntExist = errors.New("store: key does not exist")
)

type Store interface {
	Set(key string, value interface{}) error
	Get(key string) (interface{}, error)
	Delete(key string) error
	Update(key string, newValue interface{}) error
}

type MemStore struct {
	lock  *sync.Mutex
	store map[string]interface{}
}

var memStore *MemStore

func NewMemStore() Store {
	if memStore != nil {
		return memStore
	}

	memStore = &MemStore{
		lock:  new(sync.Mutex),
		store: make(map[string]interface{}),
	}

	return memStore
}

// Set is used to set a value to a key.
func (m *MemStore) Set(key string, value interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.store[key]; ok {
		return ErrKeyExists
	}
	m.store[key] = value
	return nil
}

// Get is used to get a value from a key.
func (m *MemStore) Get(key string) (interface{}, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	log.Println(m.store[key])

	if _, ok := m.store[key]; !ok {
		return nil, ErrKeyDoesntExist
	}
	return m.store[key], nil
}

// Delete removes the specified key and value.
func (m *MemStore) Delete(key string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.store[key]; !ok {
		return ErrKeyDoesntExist
	}
	delete(m.store, key)
	return nil
}

// Update can be used to change the value for a given key.
func (m *MemStore) Update(key string, value interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.store[key]; !ok {
		return ErrKeyDoesntExist
	}
	m.store[key] = value
	return nil
}
