package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"fizzbuzz-api/internal/domain"
	"fizzbuzz-api/internal/repository"
)

var (
	ctx = context.Background()
	pA  = domain.Params{Int1: 3, Int2: 5, Limit: 100, Str1: "fizz", Str2: "buzz"}
	pB  = domain.Params{Int1: 2, Int2: 4, Limit: 10, Str1: "a", Str2: "b"}
	pC  = domain.Params{Int1: 7, Int2: 9, Limit: 50, Str1: "x", Str2: "y"}
)

func TestMemoryStats_Empty(t *testing.T) {
	repo := repository.NewMemoryStats(0)

	if _, err := repo.MostFrequent(ctx); !errors.Is(err, domain.ErrNoStats) {
		t.Fatalf("expected ErrNoStats, got %v", err)
	}
}

func TestMemoryStats_MostFrequent(t *testing.T) {
	repo := repository.NewMemoryStats(0)

	for _, p := range []domain.Params{pA, pB, pB, pC, pB, pA} {
		if err := repo.Record(ctx, domain.Generation{Params: p, Path: "/api/v1/fizzbuzz"}); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	got, err := repo.MostFrequent(ctx)
	if err != nil {
		t.Fatalf("MostFrequent: %v", err)
	}
	if want := (domain.Stat{Params: pB, Hits: 3}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMemoryStats_HistoryAggregatesIdenticalRequests(t *testing.T) {
	repo := repository.NewMemoryStats(10)
	request := domain.Generation{
		Path:   "/api/v1/fizzbuzz",
		Params: pA,
	}
	if err := repo.Record(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(ctx, request); err != nil {
		t.Fatal(err)
	}

	got, err := repo.History(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Hits != 2 || got[0].Path != request.Path || got[0].Body != request.Params {
		t.Fatalf("unexpected history: %+v", got)
	}
	if got[0].RequestedAt.IsZero() || got[0].RequestedAt.Location() != time.UTC {
		t.Fatalf("requested_at must be set in UTC: %v", got[0].RequestedAt)
	}
}

func TestMemoryStats_TieKeepsFirstToReachCount(t *testing.T) {
	repo := repository.NewMemoryStats(0)

	_ = repo.Record(ctx, domain.Generation{Params: pA})
	_ = repo.Record(ctx, domain.Generation{Params: pB})
	_ = repo.Record(ctx, domain.Generation{Params: pB})
	_ = repo.Record(ctx, domain.Generation{Params: pA}) // pA and pB now both have 2 hits; pB reached 2 first

	got, _ := repo.MostFrequent(ctx)
	if got.Params != pB || got.Hits != 2 {
		t.Fatalf("got %+v, want pB with 2 hits", got)
	}
}

func TestMemoryStats_MaxEntries(t *testing.T) {
	repo := repository.NewMemoryStats(1)

	_ = repo.Record(ctx, domain.Generation{Params: pA})
	_ = repo.Record(ctx, domain.Generation{Params: pB}) // ignored: cap reached
	_ = repo.Record(ctx, domain.Generation{Params: pB})
	_ = repo.Record(ctx, domain.Generation{Params: pB})
	_ = repo.Record(ctx, domain.Generation{Params: pA}) // already tracked: still counted

	got, _ := repo.MostFrequent(ctx)
	if want := (domain.Stat{Params: pA, Hits: 2}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMemoryStats_HistoryCapacityIsReported(t *testing.T) {
	repo := repository.NewMemoryStats(1)
	if err := repo.Record(ctx, domain.Generation{Params: pA, Path: "/api/v1/fizzbuzz"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Record(ctx, domain.Generation{Params: pB, Path: "/api/v1/fizzbuzz"}); !errors.Is(err, domain.ErrHistoryCapacity) {
		t.Fatalf("expected capacity error, got %v", err)
	}
	got, err := repo.History(ctx)
	if err != nil || len(got) != 1 || got[0].Hits != 1 {
		t.Fatalf("capacity rejection must not alter history: entries=%+v err=%v", got, err)
	}
}

// Run with `go test -race` to make this test meaningful.
func TestMemoryStats_ConcurrentAccess(t *testing.T) {
	const goroutines, perGoroutine = 50, 200
	repo := repository.NewMemoryStats(0)

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				_ = repo.Record(ctx, domain.Generation{Params: pA})
				_, _ = repo.MostFrequent(ctx)
			}
		}()
	}
	wg.Wait()

	got, err := repo.MostFrequent(ctx)
	if err != nil {
		t.Fatalf("MostFrequent: %v", err)
	}
	if want := uint64(goroutines * perGoroutine); got.Hits != want {
		t.Fatalf("hits = %d, want %d", got.Hits, want)
	}
}

func BenchmarkMemoryStats_Record(b *testing.B) {
	repo := repository.NewMemoryStats(0)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = repo.Record(ctx, domain.Generation{Params: pA})
		}
	})
}
