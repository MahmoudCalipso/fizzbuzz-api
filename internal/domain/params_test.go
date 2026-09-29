package domain_test

import (
	"errors"
	"strings"
	"testing"

	"fizzbuzz-api/internal/domain"
)

func TestParams_Validate(t *testing.T) {
	valid := domain.Params{Int1: 3, Int2: 5, Limit: 100, Str1: "fizz", Str2: "buzz"}

	tests := []struct {
		name    string
		mutate  func(p *domain.Params)
		wantErr string // empty means no error
	}{
		{"valid", func(p *domain.Params) {}, ""},
		{"limit at max", func(p *domain.Params) { p.Limit = 1000 }, ""},
		{"int1 zero", func(p *domain.Params) { p.Int1 = 0 }, "int1"},
		{"int1 negative", func(p *domain.Params) { p.Int1 = -1 }, "int1"},
		{"int2 zero", func(p *domain.Params) { p.Int2 = 0 }, "int2"},
		{"limit zero", func(p *domain.Params) { p.Limit = 0 }, "limit"},
		{"limit above max", func(p *domain.Params) { p.Limit = 1001 }, "limit"},
		{"str1 empty", func(p *domain.Params) { p.Str1 = "" }, "str1"},
		{"str2 empty", func(p *domain.Params) { p.Str2 = "" }, "str2"},
		{"str1 too long", func(p *domain.Params) { p.Str1 = strings.Repeat("a", domain.MaxStringLength+1) }, "str1"},
		{"str2 too long", func(p *domain.Params) { p.Str2 = strings.Repeat("é", domain.MaxStringLength+1) }, "str2"},
		{"str2 multibyte at max", func(p *domain.Params) { p.Str2 = strings.Repeat("é", domain.MaxStringLength) }, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := valid
			tc.mutate(&p)

			err := p.Validate(1000)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			var vErr *domain.ValidationError
			if !errors.As(err, &vErr) {
				t.Fatalf("expected *ValidationError, got %T (%v)", err, err)
			}
			if !strings.Contains(vErr.Error(), tc.wantErr) {
				t.Fatalf("error %q should mention %q", vErr.Error(), tc.wantErr)
			}
		})
	}
}
