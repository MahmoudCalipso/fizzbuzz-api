// Package controller is the HTTP adapter (the "controller" layer): it turns
// HTTP requests into calls to the use cases and turns use-case results and
// errors into HTTP responses. It contains no business logic.
package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/xuri/excelize/v2"

	"fizzbuzz-api/internal/domain"
)

// maxBodyBytes protects the server against oversized request bodies.
const maxBodyBytes = 4 << 10 // 4 KiB

// UseCase is the port the controller depends on. It is declared here, on the
// consumer side, so the controller can be tested with a trivial fake.
type UseCase interface {
	Generate(ctx context.Context, request domain.Generation) ([]string, error)
	MostFrequent(ctx context.Context) (domain.Stat, error)
	History(ctx context.Context) ([]domain.HistoryEntry, error)
}

// Controller exposes the use cases over HTTP.
type Controller struct {
	uc  UseCase
	log *slog.Logger
}

// FizzBuzzRequest is the JSON body of POST /fizzbuzz.
//
// Fields are pointers so a missing field (nil, rejected by `required`) can be
// told apart from an explicit zero value (rejected later by domain validation
// with a precise message).
type FizzBuzzRequest struct {
	Int1  *int    `json:"int1"  binding:"required" minimum:"1" example:"3"`
	Int2  *int    `json:"int2"  binding:"required" minimum:"1" example:"5"`
	Limit *int    `json:"limit" binding:"required" minimum:"1" example:"100"`
	Str1  *string `json:"str1"  binding:"required" maxLength:"64" example:"fizz"`
	Str2  *string `json:"str2"  binding:"required" maxLength:"64" example:"buzz"`
}

// StatsResponse is the JSON body of GET /stats.
type StatsResponse struct {
	Int1  int    `json:"int1"  example:"3"`
	Int2  int    `json:"int2"  example:"5"`
	Limit int    `json:"limit" example:"100"`
	Str1  string `json:"str1"  example:"fizz"`
	Str2  string `json:"str2"  example:"buzz"`
	Hits  uint64 `json:"hits"  example:"42"`
}

// ErrorResponse is the JSON body of every error.
type ErrorResponse struct {
	Error string `json:"error" example:"int1 must be a positive integer"`
}

// FizzBuzz godoc
//
//	@Summary		Generate a fizz-buzz list
//	@Description	Returns the numbers from 1 to `limit` where multiples of `int1` are replaced by `str1`, multiples of `int2` by `str2`, and multiples of both by `str1str2`.
//	@Tags			fizzbuzz
//	@Accept			json
//	@Produce		json
//	@Param			request	body		FizzBuzzRequest	true	"Fizz-buzz parameters"
//	@Success		200		{array}		string
//	@Failure		400		{object}	ErrorResponse
//	@Failure		503		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/fizzbuzz [post]
func (h *Controller) fizzBuzz(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)

	var req FizzBuzzRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: describeBindError(err)})
		return
	}

	request := domain.Generation{
		Path: c.Request.URL.Path,
		Params: domain.Params{
			Int1:  *req.Int1,
			Int2:  *req.Int2,
			Limit: *req.Limit,
			Str1:  *req.Str1,
			Str2:  *req.Str2,
		},
	}
	result, err := h.uc.Generate(c.Request.Context(), request)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// history godoc
//
//	@Summary		Generation request history
//	@Description	Returns successful generation requests grouped by URL path and validated parameters, newest request first. requested_at is the UTC time of the latest matching request.
//	@Tags			stats
//	@Produce		json
//	@Success		200	{array}		domain.HistoryEntry
//	@Failure		500	{object}	ErrorResponse
//	@Router		/history [get]
func (h *Controller) history(c *gin.Context) {
	entries, err := h.uc.History(c.Request.Context())
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, entries)
}

// exportHistory godoc
//
//	@Summary		Export generation history to Excel
//	@Description	Downloads all tracked successful generation request groups as an XLSX spreadsheet.
//	@Tags			stats
//	@Produce		application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
//	@Success		200	{file}		binary
//	@Failure		500	{object}	ErrorResponse
//	@Router		/history/export [get]
func (h *Controller) exportHistory(c *gin.Context) {
	entries, err := h.uc.History(c.Request.Context())
	if err != nil {
		h.writeError(c, err)
		return
	}
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	const sheet = "History"
	if err := file.SetSheetName("Sheet1", sheet); err != nil {
		h.writeError(c, fmt.Errorf("creating history workbook: %w", err))
		return
	}
	stream, err := file.NewStreamWriter(sheet)
	if err != nil {
		h.writeError(c, fmt.Errorf("creating history worksheet: %w", err))
		return
	}
	headers := []any{"Path URL", "Body", "Request time (UTC)", "Hits"}
	for _, width := range []struct {
		column int
		width  float64
	}{{1, 28}, {2, 58}, {3, 32}, {4, 12}} {
		if err := stream.SetColWidth(width.column, width.column, width.width); err != nil {
			h.writeError(c, fmt.Errorf("formatting history worksheet: %w", err))
			return
		}
	}
	styleID, err := file.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		h.writeError(c, fmt.Errorf("formatting history headers: %w", err))
		return
	}
	if err := stream.SetRow("A1", headers, excelize.RowOpts{StyleID: styleID}); err != nil {
		h.writeError(c, fmt.Errorf("writing history headers: %w", err))
		return
	}
	for i, entry := range entries {
		body, err := json.Marshal(entry.Body)
		if err != nil {
			h.writeError(c, fmt.Errorf("encoding history body: %w", err))
			return
		}
		row := []any{entry.Path, string(body), entry.RequestedAt.UTC().Format(time.RFC3339Nano), entry.Hits}
		if err := stream.SetRow(fmt.Sprintf("A%d", i+2), row); err != nil {
			h.writeError(c, fmt.Errorf("writing history row: %w", err))
			return
		}
	}
	if err := stream.Flush(); err != nil {
		h.writeError(c, fmt.Errorf("finishing history worksheet: %w", err))
		return
	}
	var workbook bytes.Buffer
	if err := file.Write(&workbook); err != nil {
		h.writeError(c, fmt.Errorf("encoding history workbook: %w", err))
		return
	}
	c.Header("Content-Disposition", `attachment; filename="fizzbuzz-history.xlsx"`)
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", workbook.Bytes())
}

// stats godoc
//
//	@Summary		Most frequent request
//	@Description	Returns the parameters of the most frequent valid request and its number of hits. On a tie, the request that reached the count first is returned.
//	@Tags			stats
//	@Produce		json
//	@Success		200	{object}	StatsResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/stats [get]
func (h *Controller) stats(c *gin.Context) {
	s, err := h.uc.MostFrequent(c.Request.Context())
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, StatsResponse{
		Int1:  s.Params.Int1,
		Int2:  s.Params.Int2,
		Limit: s.Params.Limit,
		Str1:  s.Params.Str1,
		Str2:  s.Params.Str2,
		Hits:  s.Hits,
	})
}

// health handles GET /healthz (liveness / readiness probe).
func (h *Controller) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// describeBindError turns a binding error into a client-friendly message.
func describeBindError(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := make([]string, 0, len(ve))
		for _, fe := range ve {
			fields = append(fields, strings.ToLower(fe.Field())) // struct field names match the JSON names once lowercased
		}
		return "missing required fields: " + strings.Join(fields, ", ")
	}
	return "invalid request body: " + err.Error()
}

// writeError maps domain errors to HTTP status codes. Unexpected errors are
// logged and never leaked to the client.
func (h *Controller) writeError(c *gin.Context, err error) {
	var vErr *domain.ValidationError
	switch {
	case errors.As(err, &vErr):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: vErr.Error()})
	case errors.Is(err, domain.ErrNoStats):
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrHistoryCapacity):
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
	default:
		h.log.Error("unexpected error", "path", c.FullPath(), "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}
}
