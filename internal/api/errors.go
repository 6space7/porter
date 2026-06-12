package api

import (
	"encoding/json"
	"net/http"
)

type ErrorEnvelope struct {
	Error ErrorResponse `json:"error"`
}

type ErrorResponse struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Hint    string         `json:"hint"`
	Details map[string]any `json:"details"`
}

func WriteError(w http.ResponseWriter, status int, code, message, hint string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorEnvelope{
		Error: ErrorResponse{
			Code:    code,
			Message: message,
			Hint:    hint,
			Details: details,
		},
	})
}
