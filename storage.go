package main

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrNotFound      = errors.New("host not found")
	ErrAlreadyExists = errors.New("host ID already exists")
)

type Store interface {
	Register(host *RegisteredHost) error
	Get(id string) (*RegisteredHost, error)
	List() ([]*HostListItem, error)
	Delete(id string) error
	SetAnswer(id string, answer string) error
}

type MemoryStore struct {
	mu    sync.RWMutex
	hosts map[string]*RegisteredHost
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		hosts: make(map[string]*RegisteredHost),
	}
}

func (s *MemoryStore) Register(host *RegisteredHost) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.hosts[host.ID]; exists {
		return ErrAlreadyExists
	}

	host.RegisteredAt = time.Now()
	host.LastSeen = host.RegisteredAt
	s.hosts[host.ID] = host
	return nil
}

func (s *MemoryStore) Get(id string) (*RegisteredHost, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	host, exists := s.hosts[id]
	if !exists {
		return nil, ErrNotFound
	}
	return host, nil
}

func (s *MemoryStore) List() ([]*HostListItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*HostListItem, 0, len(s.hosts))
	for _, host := range s.hosts {
		items = append(items, &HostListItem{
			ID:           host.ID,
			RegisteredAt: host.RegisteredAt,
			RemoteAddr:   host.RemoteAddr,
		})
	}
	return items, nil
}

func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.hosts[id]; !exists {
		return ErrNotFound
	}
	delete(s.hosts, id)
	return nil
}

func (s *MemoryStore) SetAnswer(id string, answer string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	host, exists := s.hosts[id]
	if !exists {
		return ErrNotFound
	}
	host.Answer = answer
	return nil
}
