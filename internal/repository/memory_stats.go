// Package repository contains the adapters implementing the domain ports.
package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"fizzbuzz-api/internal/domain"
)

// MemoryStats is a thread-safe, in-memory implementation of
// domain.StatsRepository.
//
// Record and MostFrequent are both O(1): the current winner is updated on
// every write, so reads never have to scan the map.
//
// To protect the process from unbounded memory growth (e.g. a client sending
// endless distinct parameter combinations), at most maxEntries distinct
// parameter sets and request bodies are tracked. New requests beyond the cap
// return domain.ErrHistoryCapacity; existing request groups continue counting.
type MemoryStats struct {
	mu         sync.RWMutex
	counts     map[domain.Params]uint64
	history    map[historyKey]domain.HistoryEntry
	best       domain.Stat
	maxEntries int
}

type historyKey struct {
	path   string
	params domain.Params
}

var _ domain.StatsRepository = (*MemoryStats)(nil)

// NewMemoryStats creates an empty repository. maxEntries <= 0 means unbounded.
func NewMemoryStats(maxEntries int) *MemoryStats {
	return &MemoryStats{
		counts:     make(map[domain.Params]uint64),
		history:    make(map[historyKey]domain.HistoryEntry),
		maxEntries: maxEntries,
	}
}

// Record registers one more hit for the request and updates the matching history group.
func (m *MemoryStats) Record(_ context.Context, generation domain.Generation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p := generation.Params
	key := historyKey{path: generation.Path, params: p}
	entry, exists := m.history[key]
	_, knownParams := m.counts[p]
	if m.maxEntries > 0 && ((!knownParams && len(m.counts) >= m.maxEntries) || (!exists && len(m.history) >= m.maxEntries)) {
		return domain.ErrHistoryCapacity
	}

	m.counts[p]++
	// Strictly greater: on a tie, the combination that reached the count first wins.
	if c := m.counts[p]; c > m.best.Hits {
		m.best = domain.Stat{Params: p, Hits: c}
	}

	if !exists {
		entry = domain.HistoryEntry{Path: generation.Path, Body: p}
	}
	entry.RequestedAt = time.Now().UTC()
	entry.Hits++
	m.history[key] = entry
	return nil
}

// MostFrequent returns the most requested parameters, or domain.ErrNoStats.
func (m *MemoryStats) MostFrequent(_ context.Context) (domain.Stat, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.best.Hits == 0 {
		return domain.Stat{}, domain.ErrNoStats
	}
	return m.best, nil
}

// History returns a snapshot of tracked request groups, newest first.
func (m *MemoryStats) History(_ context.Context) ([]domain.HistoryEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]domain.HistoryEntry, 0, len(m.history))
	for _, entry := range m.history {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].RequestedAt.Equal(entries[j].RequestedAt) {
			if entries[i].Path == entries[j].Path {
				return fmt.Sprint(entries[i].Body) < fmt.Sprint(entries[j].Body)
			}
			return entries[i].Path < entries[j].Path
		}
		return entries[i].RequestedAt.After(entries[j].RequestedAt)
	})
	return entries, nil
}
