// Package service holds the business logic (use cases) of the application.
package service

import (
	"context"
	"fmt"
	"strconv"

	"fizzbuzz-api/internal/domain"
)

// FizzBuzz implements the fizz-buzz and statistics use cases.
type FizzBuzz struct {
	stats    domain.StatsRepository
	maxLimit int
}

// New builds a FizzBuzz service. maxLimit is the largest accepted `limit`.
func New(stats domain.StatsRepository, maxLimit int) *FizzBuzz {
	return &FizzBuzz{stats: stats, maxLimit: maxLimit}
}

// Generate validates the parameters, builds the fizz-buzz list and records
// the request for statistics. Invalid requests are never recorded.

func (s *FizzBuzz) Generate(ctx context.Context, request domain.Generation) ([]string, error) {
	p := request.Params
	if err := p.Validate(s.maxLimit); err != nil {
		return nil, err
	}

	result := compute(p)

	if err := s.stats.Record(ctx, request); err != nil {
		return nil, fmt.Errorf("recording statistics: %w", err)
	}
	return result, nil
}

// History returns the tracked successful generation requests.
func (s *FizzBuzz) History(ctx context.Context) ([]domain.HistoryEntry, error) {
	return s.stats.History(ctx)
}

// MostFrequent returns the most requested parameters and their hit count.
func (s *FizzBuzz) MostFrequent(ctx context.Context) (domain.Stat, error) {
	return s.stats.MostFrequent(ctx)
}

// compute is a pure function: it assumes p has already been validated.
func compute(p domain.Params) []string {
	out := make([]string, p.Limit) // single allocation for the slice
	both := p.Str1 + p.Str2        // computed once, not per iteration

	for i := 1; i <= p.Limit; i++ {
		m1 := i%p.Int1 == 0
		m2 := i%p.Int2 == 0

		switch {
		case m1 && m2:
			out[i-1] = both
		case m1:
			out[i-1] = p.Str1
		case m2:
			out[i-1] = p.Str2
		default:
			out[i-1] = strconv.Itoa(i)
		}
	}
	return out
}
