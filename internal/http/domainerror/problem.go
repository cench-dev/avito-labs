package domainerror

import (
	"encoding/json"
	"errors"
	"net/http"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	api "github.com/cench-dev/avito-labs/internal/generated"
)

func Write(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, tripDomain.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "not_found", "Not Found", "trip not found")

	case errors.Is(err, tripDomain.ErrAlreadyCompleted):
		writeProblem(w, r, http.StatusConflict, "trip_already_completed", "Conflict", "trip is already completed")

	case errors.Is(err, tripDomain.ErrDriverBusy):
		writeProblem(w, r, http.StatusConflict, "driver_busy", "Conflict", "driver already has an active trip")

	default:
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "an internal error occurred")
	}
}

func WriteProblem(w http.ResponseWriter, r *http.Request, status int,
	code string, title string, detail string) {
	writeProblem(w, r, status, code, title, detail)
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int,
	code string, title string, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	problem := api.Problem{
		Code:     code,
		Status:   int32(status),
		Title:    title,
		Type:     "about:blank",
		Instance: &r.URL.Path,
	}

	if detail != "" {
		problem.Detail = &detail
	}

	_ = json.NewEncoder(w).Encode(problem)
}
