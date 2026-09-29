package controller_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"fizzbuzz-api/internal/controller"
	"fizzbuzz-api/internal/repository"
	"fizzbuzz-api/internal/service"
)

// TestEndToEnd exercises every layer together (no mocks).
func TestEndToEnd(t *testing.T) {
	srv := httptest.NewServer(controller.NewRouter(
		service.New(repository.NewMemoryStats(0), 1000),
		discardLogger(),
	))
	defer srv.Close()

	send := func(method, path, body string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatalf("building request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		defer resp.Body.Close()
		buf, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading body of %s %s: %v", method, path, err)
		}
		return resp.StatusCode, buf
	}

	const (
		classic = `{"int1":3,"int2":5,"limit":15,"str1":"fizz","str2":"buzz"}`
		other   = `{"int1":2,"int2":3,"limit":6,"str1":"a","str2":"b"}`
		invalid = `{"int1":0,"int2":5,"limit":10,"str1":"a","str2":"b"}`
	)

	// No traffic yet.
	if status, _ := send(http.MethodGet, "/api/v1/stats", ""); status != http.StatusNotFound {
		t.Fatalf("stats before traffic: status = %d, want 404", status)
	}

	// An invalid request is rejected and must not be counted.
	if status, _ := send(http.MethodPost, "/api/v1/fizzbuzz", invalid); status != http.StatusBadRequest {
		t.Fatalf("invalid request: status = %d, want 400", status)
	}

	// Real answer for the classic case.
	status, body := send(http.MethodPost, "/api/v1/fizzbuzz", classic)
	if status != http.StatusOK {
		t.Fatalf("classic request: status = %d", status)
	}
	var list []string
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("invalid body: %v", err)
	}
	want := []string{"1", "2", "fizz", "4", "buzz", "fizz", "7", "8", "fizz", "buzz", "11", "fizz", "13", "14", "fizzbuzz"}
	if !reflect.DeepEqual(list, want) {
		t.Fatalf("got %v, want %v", list, want)
	}

	for _, b := range []string{other, classic} {
		if status, _ := send(http.MethodPost, "/api/v1/fizzbuzz", b); status != http.StatusOK {
			t.Fatalf("request %s: status = %d", b, status)
		}
	}

	status, body = send(http.MethodGet, "/api/v1/stats", "")
	if status != http.StatusOK {
		t.Fatalf("stats: status = %d", status)
	}
	var stat map[string]any
	if err := json.Unmarshal(body, &stat); err != nil {
		t.Fatalf("stats: invalid body: %v", err)
	}
	if stat["hits"] != 2.0 || stat["int1"] != 3.0 || stat["int2"] != 5.0 || stat["limit"] != 15.0 ||
		stat["str1"] != "fizz" || stat["str2"] != "buzz" {
		t.Fatalf("unexpected stats: %v", stat)
	}

	status, body = send(http.MethodGet, "/api/v1/history", "")
	if status != http.StatusOK {
		t.Fatalf("history: status = %d, body = %s", status, body)
	}
	var history []struct {
		Path        string          `json:"path_url"`
		Body        json.RawMessage `json:"body"`
		RequestedAt string          `json:"requested_at"`
		Hits        uint64          `json:"hits"`
	}
	if err := json.Unmarshal(body, &history); err != nil {
		t.Fatalf("history: invalid body: %v", err)
	}
	if len(history) != 2 || history[0].Path != "/api/v1/fizzbuzz" || history[1].Path != "/api/v1/fizzbuzz" {
		t.Fatalf("unexpected history: %+v", history)
	}
	totalHits := history[0].Hits + history[1].Hits
	if totalHits != 3 || history[0].RequestedAt == "" || history[1].RequestedAt == "" {
		t.Fatalf("unexpected history counts or timestamps: %+v", history)
	}

	status, body = send(http.MethodGet, "/api/v1/history/export", "")
	if status != http.StatusOK || !bytes.HasPrefix(body, []byte("PK")) {
		t.Fatalf("history export: status = %d, body is not an XLSX file", status)
	}
}
