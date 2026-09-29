package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"fizzbuzz-api/internal/domain"
)

// fakeRepo is a hand-written test double for domain.StatsRepository.
type fakeRepo struct {
	recorded   []domain.Params
	recordErr  error
	stat       domain.Stat
	statErr    error
	history    []domain.HistoryEntry
	historyErr error
}

func (f *fakeRepo) Record(_ context.Context, request domain.Generation) error {
	if f.recordErr != nil {
		return f.recordErr
	}
	f.recorded = append(f.recorded, request.Params)
	return nil
}

func (f *fakeRepo) MostFrequent(context.Context) (domain.Stat, error) {
	return f.stat, f.statErr
}

func (f *fakeRepo) History(context.Context) ([]domain.HistoryEntry, error) {
	return f.history, f.historyErr
}

func TestCompute(t *testing.T) {
	tests := []struct {
		name string
		p    domain.Params
		want []string
	}{
		{
			name: "classic fizzbuzz",
			p:    domain.Params{Int1: 3, Int2: 5, Limit: 15, Str1: "fizz", Str2: "buzz"},
			want: []string{"1", "2", "fizz", "4", "buzz", "fizz", "7", "8", "fizz", "buzz", "11", "fizz", "13", "14", "fizzbuzz"},
		},
		{
			name: "custom values",
			p:    domain.Params{Int1: 2, Int2: 4, Limit: 8, Str1: "a", Str2: "b"},
			want: []string{"1", "a", "3", "ab", "5", "a", "7", "ab"},
		},
		{
			name: "same divisor for both",
			p:    domain.Params{Int1: 2, Int2: 2, Limit: 4, Str1: "x", Str2: "y"},
			want: []string{"1", "xy", "3", "xy"},
		},
		{
			name: "divisor 1 replaces everything",
			p:    domain.Params{Int1: 1, Int2: 3, Limit: 3, Str1: "n", Str2: "t"},
			want: []string{"n", "n", "nt"},
		},
		{
			name: "limit 1",
			p:    domain.Params{Int1: 3, Int2: 5, Limit: 1, Str1: "fizz", Str2: "buzz"},
			want: []string{"1"},
		},
		{
			name: "divisors larger than limit",
			p:    domain.Params{Int1: 10, Int2: 20, Limit: 5, Str1: "a", Str2: "b"},
			want: []string{"1", "2", "3", "4", "5"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := compute(tc.p); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("compute() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGenerate_Success_RecordsRequest(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, 100)
	p := domain.Params{Int1: 3, Int2: 5, Limit: 5, Str1: "fizz", Str2: "buzz"}

	got, err := svc.Generate(context.Background(), domain.Generation{Params: p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"1", "2", "fizz", "4", "buzz"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if len(repo.recorded) != 1 || repo.recorded[0] != p {
		t.Fatalf("expected exactly one recorded request %+v, got %+v", p, repo.recorded)
	}
}

func TestGenerate_InvalidParams_NotRecorded(t *testing.T) {
	repo := &fakeRepo{}
	svc := New(repo, 100)

	_, err := svc.Generate(context.Background(), domain.Generation{Params: domain.Params{Int1: 0, Int2: 5, Limit: 5, Str1: "a", Str2: "b"}})

	var vErr *domain.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *ValidationError, got %v", err)
	}
	if len(repo.recorded) != 0 {
		t.Fatalf("invalid request must not be recorded, got %+v", repo.recorded)
	}
}

func TestGenerate_LimitAboveMax(t *testing.T) {
	svc := New(&fakeRepo{}, 10)

	_, err := svc.Generate(context.Background(), domain.Generation{Params: domain.Params{Int1: 1, Int2: 2, Limit: 11, Str1: "a", Str2: "b"}})

	var vErr *domain.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *ValidationError, got %v", err)
	}
}

func TestGenerate_RepositoryError(t *testing.T) {
	boom := errors.New("boom")
	svc := New(&fakeRepo{recordErr: boom}, 100)

	_, err := svc.Generate(context.Background(), domain.Generation{Params: domain.Params{Int1: 3, Int2: 5, Limit: 5, Str1: "a", Str2: "b"}})

	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped repository error, got %v", err)
	}
}

func TestMostFrequent_Delegates(t *testing.T) {
	want := domain.Stat{Params: domain.Params{Int1: 3, Int2: 5, Limit: 100, Str1: "fizz", Str2: "buzz"}, Hits: 7}
	svc := New(&fakeRepo{stat: want}, 100)

	got, err := svc.MostFrequent(context.Background())
	if err != nil || got != want {
		t.Fatalf("got (%+v, %v), want (%+v, nil)", got, err, want)
	}

	svc = New(&fakeRepo{statErr: domain.ErrNoStats}, 100)
	if _, err := svc.MostFrequent(context.Background()); !errors.Is(err, domain.ErrNoStats) {
		t.Fatalf("expected ErrNoStats, got %v", err)
	}
}

func BenchmarkCompute(b *testing.B) {
	p := domain.Params{Int1: 3, Int2: 5, Limit: 10000, Str1: "fizz", Str2: "buzz"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = compute(p)
	}
}
