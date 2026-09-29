// Package domain contains the core business types and the ports (interfaces)
// the rest of the application depends on. It has no external dependencies.
package domain

import (
	"fmt"
	"unicode/utf8"
)

// MaxStringLength is the maximum number of characters allowed for str1 / str2.
const MaxStringLength = 64

// Params describes one fizz-buzz request. It is comparable, so it can be used
// directly as a map key by the statistics repository.
type Params struct {
	Int1  int    `json:"int1"`
	Int2  int    `json:"int2"`
	Limit int    `json:"limit"`
	Str1  string `json:"str1"`
	Str2  string `json:"str2"`
}

// ValidationError is returned when the caller supplied invalid parameters.
type ValidationError struct {
	Reason string
}

func (e *ValidationError) Error() string { return e.Reason }

// Validate checks business rules. maxLimit protects the server against
// requests that would generate unreasonably large responses.
func (p Params) Validate(maxLimit int) error {
	switch {
	case p.Int1 <= 0:
		return &ValidationError{Reason: "int1 must be a positive integer"}
	case p.Int2 <= 0:
		return &ValidationError{Reason: "int2 must be a positive integer"}
	case p.Limit <= 0:
		return &ValidationError{Reason: "limit must be a positive integer"}
	case p.Limit > maxLimit:
		return &ValidationError{Reason: fmt.Sprintf("limit must be <= %d", maxLimit)}
	case p.Str1 == "":
		return &ValidationError{Reason: "str1 must not be empty"}
	case p.Str2 == "":
		return &ValidationError{Reason: "str2 must not be empty"}
	case utf8.RuneCountInString(p.Str1) > MaxStringLength:
		return &ValidationError{Reason: fmt.Sprintf("str1 must be at most %d characters", MaxStringLength)}
	case utf8.RuneCountInString(p.Str2) > MaxStringLength:
		return &ValidationError{Reason: fmt.Sprintf("str2 must be at most %d characters", MaxStringLength)}
	}
	return nil
}
