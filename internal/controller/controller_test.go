package controller_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"fizzbuzz-api/internal/controller"
	"fizzbuzz-api/internal/domain"
)

func init() { gin.SetMode(gin.TestMode) }

const validBody = `{"int1":3,"int2":5,"limit":3,"str1":"fizz","str2":"buzz"}`

// fakeUseCase is a test double for controller.UseCase.
type fakeUseCase struct {
	called     bool
	gotParams  domain.Params
	request    domain.Generation
	result     []string
	genErr     error
	stat       domain.Stat
	statErr    error
	history    []domain.HistoryEntry
	historyErr error
}

func (f *fakeUseCase) Generate(_ context.Context, request domain.Generation) ([]string, error) {
	f.called = true
	f.gotParams = request.Params
	f.request = request
	return f.result, f.genErr
}

func (f *fakeUseCase) MostFrequent(context.Context) (domain.Stat, error) {
	return f.stat, f.statErr
}

func (f *fakeUseCase) History(context.Context) ([]domain.HistoryEntry, error) {
	return f.history, f.historyErr
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func do(t *testing.T, uc controller.UseCase, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	controller.NewRouter(uc, discardLogger()).ServeHTTP(rec, req)
	return rec
}

func TestFizzBuzz_OK(t *testing.T) {
	uc := &fakeUseCase{result: []string{"1", "2", "fizz"}}

	rec := do(t, uc, http.MethodPost, "/api/v1/fizzbuzz", validBody)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body)
	}
	var got []string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not a JSON string array: %v", err)
	}
	if len(got) != 3 || got[2] != "fizz" {
		t.Fatalf("unexpected body: %v", got)
	}
	want := domain.Params{Int1: 3, Int2: 5, Limit: 3, Str1: "fizz", Str2: "buzz"}
	if uc.gotParams != want {
		t.Fatalf("use case received %+v, want %+v", uc.gotParams, want)
	}
	if uc.request.Path != "/api/v1/fizzbuzz" || uc.request.Params != want {
		t.Fatalf("use case received generation metadata %+v", uc.request)
	}
}

func TestFizzBuzz_BadRequests(t *testing.T) {
	tests := []struct{ name, body string }{
		{"empty body", ""},
		{"malformed json", `{"int1":`},
		{"empty object", `{}`},
		{"missing str2", `{"int1":3,"int2":5,"limit":10,"str1":"fizz"}`},
		{"missing limit", `{"int1":3,"int2":5,"str1":"fizz","str2":"buzz"}`},
		{"string instead of int", `{"int1":"abc","int2":5,"limit":10,"str1":"a","str2":"b"}`},
		{"float instead of int", `{"int1":3,"int2":5,"limit":1.5,"str1":"a","str2":"b"}`},
		{"int instead of string", `{"int1":3,"int2":5,"limit":10,"str1":1,"str2":"b"}`},
		{"body too large", `{"int1":3,"int2":5,"limit":10,"str1":"` + strings.Repeat("a", 5000) + `","str2":"b"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := &fakeUseCase{}

			rec := do(t, uc, http.MethodPost, "/api/v1/fizzbuzz", tc.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] == "" {
				t.Fatalf(`expected {"error": ...}, got %s`, rec.Body)
			}
			if uc.called {
				t.Fatal("use case must not be called for a malformed request")
			}
		})
	}
}

func TestFizzBuzz_MissingFieldsAreNamed(t *testing.T) {
	rec := do(t, &fakeUseCase{}, http.MethodPost, "/api/v1/fizzbuzz", `{"int1":3,"int2":5,"limit":10}`)

	msg := rec.Body.String()
	if !strings.Contains(msg, "str1") || !strings.Contains(msg, "str2") {
		t.Fatalf("error should name the missing fields, got %s", msg)
	}
}

func TestFizzBuzz_ExplicitZeroReachesUseCase(t *testing.T) {
	// 0 is present (so it is not "missing"): the domain layer rejects it with a precise message.
	uc := &fakeUseCase{genErr: &domain.ValidationError{Reason: "int1 must be a positive integer"}}

	rec := do(t, uc, http.MethodPost, "/api/v1/fizzbuzz", `{"int1":0,"int2":5,"limit":3,"str1":"a","str2":"b"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !uc.called || uc.gotParams.Int1 != 0 {
		t.Fatalf("use case should have received int1=0, got called=%v params=%+v", uc.called, uc.gotParams)
	}
	if !strings.Contains(rec.Body.String(), "int1 must be a positive integer") {
		t.Fatalf("unexpected body: %s", rec.Body)
	}
}

func TestFizzBuzz_UnexpectedErrorIsNotLeaked(t *testing.T) {
	rec := do(t, &fakeUseCase{genErr: errors.New("db exploded")}, http.MethodPost, "/api/v1/fizzbuzz", validBody)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "db exploded") {
		t.Fatal("internal error details must not leak to the client")
	}
}

func TestFizzBuzz_GETNotAllowed(t *testing.T) {
	rec := do(t, &fakeUseCase{}, http.MethodGet, "/api/v1/fizzbuzz?int1=3&int2=5&limit=3&str1=a&str2=b", "")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestStats_OK(t *testing.T) {
	uc := &fakeUseCase{stat: domain.Stat{
		Params: domain.Params{Int1: 3, Int2: 5, Limit: 100, Str1: "fizz", Str2: "buzz"},
		Hits:   42,
	}}

	rec := do(t, uc, http.MethodGet, "/api/v1/stats", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	want := map[string]any{"int1": 3.0, "int2": 5.0, "limit": 100.0, "str1": "fizz", "str2": "buzz", "hits": 42.0}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
}

func TestStats_NoData(t *testing.T) {
	rec := do(t, &fakeUseCase{statErr: domain.ErrNoStats}, http.MethodGet, "/api/v1/stats", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHistory_ReturnsEntries(t *testing.T) {
	uc := &fakeUseCase{history: []domain.HistoryEntry{{
		Path:        "/api/v1/fizzbuzz",
		Body:        domain.Params{Int1: 3, Int2: 5, Limit: 3, Str1: "fizz", Str2: "buzz"},
		RequestedAt: time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC),
		Hits:        3,
	}}}
	rec := do(t, uc, http.MethodGet, "/api/v1/history", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var entries []domain.HistoryEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "/api/v1/fizzbuzz" || entries[0].Hits != 3 || entries[0].Body != (domain.Params{Int1: 3, Int2: 5, Limit: 3, Str1: "fizz", Str2: "buzz"}) {
		t.Fatalf("unexpected history: %+v", entries)
	}
}

func TestHistoryExport_ReturnsExcelWorkbook(t *testing.T) {
	uc := &fakeUseCase{history: []domain.HistoryEntry{{
		Path:        "/api/v1/fizzbuzz",
		Body:        domain.Params{Int1: 3, Int2: 5, Limit: 3, Str1: "fizz", Str2: "buzz"},
		RequestedAt: time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC),
		Hits:        3,
	}}}
	rec := do(t, uc, http.MethodGet, "/api/v1/history/export", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("content type = %q", got)
	}
	book, err := excelize.OpenReader(rec.Body)
	if err != nil {
		t.Fatalf("response is not a valid Excel workbook: %v", err)
	}
	defer book.Close()
	rows, err := book.GetRows("History")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0][0] != "Path URL" || rows[0][3] != "Hits" || rows[1][0] != "/api/v1/fizzbuzz" || rows[1][3] != "3" {
		t.Fatalf("unexpected worksheet: %v", rows)
	}
}

func TestGenerate_HistoryCapacityReturnsServiceUnavailable(t *testing.T) {
	rec := do(t, &fakeUseCase{genErr: domain.ErrHistoryCapacity}, http.MethodPost, "/api/v1/fizzbuzz", validBody)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestStats_UnexpectedError(t *testing.T) {
	rec := do(t, &fakeUseCase{statErr: errors.New("boom")}, http.MethodGet, "/api/v1/stats", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestHealth(t *testing.T) {
	rec := do(t, &fakeUseCase{}, http.MethodGet, "/healthz", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestSwaggerSpecIsServed(t *testing.T) {
	rec := do(t, &fakeUseCase{}, http.MethodGet, "/swagger/doc.json", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("spec is not valid JSON: %v", err)
	}
	if _, ok := doc.Paths["/fizzbuzz"]["post"]; !ok {
		t.Error("spec must document POST /fizzbuzz")
	}
	if _, ok := doc.Paths["/stats"]["get"]; !ok {
		t.Error("spec must document GET /stats")
	}
	if _, ok := doc.Paths["/history"]["get"]; !ok {
		t.Error("spec must document GET /history")
	}
	if _, ok := doc.Paths["/history/export"]["get"]; !ok {
		t.Error("spec must document GET /history/export")
	}
}
