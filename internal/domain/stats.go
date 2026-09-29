package domain

import (
	"context"
	"errors"
	"time"
)

// ErrNoStats is returned when no request has been recorded yet.
var ErrNoStats = errors.New("no statistics available yet")

// ErrHistoryCapacity indicates that the in-memory request history reached its configured limit.
var ErrHistoryCapacity = errors.New("request history capacity reached")

// Stat associates a set of request parameters with its number of hits.
type Stat struct {
	Params Params
	Hits   uint64
}

// Generation records the HTTP request associated with a successful generation.
type Generation struct {
	Path   string
	Params Params
}

// HistoryEntry aggregates successful generations with the same path and body.
type HistoryEntry struct {
	Path        string    `json:"path_url"`
	Body        Params    `json:"body"`
	RequestedAt time.Time `json:"requested_at"`
	Hits        uint64    `json:"hits"`
}

// StatsRepository is the persistence port for request statistics.
// Implementations must be safe for concurrent use.
type StatsRepository interface {
	// Record registers a successful generation and its request details.
	Record(ctx context.Context, generation Generation) error
	// MostFrequent returns the most requested parameters, or ErrNoStats.
	MostFrequent(ctx context.Context) (Stat, error)
	// History returns all tracked request groups, newest first.
	History(ctx context.Context) ([]HistoryEntry, error)
}
