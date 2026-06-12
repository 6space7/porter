package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/6space7/porter/internal/api"
)

func TestWriteErrorUsesMachineLegibleEnvelope(t *testing.T) {
	rr := httptest.NewRecorder()

	api.WriteError(
		rr,
		http.StatusBadRequest,
		"invalid_name",
		"Name is invalid.",
		"Use lowercase letters, numbers, and hyphens.",
		map[string]any{"field": "name"},
	)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}

	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Hint    string         `json:"hint"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}

	if body.Error.Code != "invalid_name" {
		t.Fatalf("code = %q, want invalid_name", body.Error.Code)
	}
	if body.Error.Message != "Name is invalid." {
		t.Fatalf("message = %q, want Name is invalid.", body.Error.Message)
	}
	if body.Error.Hint != "Use lowercase letters, numbers, and hyphens." {
		t.Fatalf("hint = %q", body.Error.Hint)
	}
	if body.Error.Details["field"] != "name" {
		t.Fatalf("details[field] = %#v, want name", body.Error.Details["field"])
	}
}
