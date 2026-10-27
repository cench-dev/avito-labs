package get

import (
	"encoding/json"
	"net/http"

	api "github.com/cench-dev/avito-labs/internal/generated"
	"github.com/cench-dev/avito-labs/internal/http/domainerror"
	tripConverter "github.com/cench-dev/avito-labs/internal/http/trip"
)

type Handler struct {
	useCase TripGetter
}

func NewHandler(useCase TripGetter) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	id := tripId.String()

	trip, err := h.useCase.Get(r.Context(), id)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	apiTrip, err := tripConverter.ConvertAPITrip(trip)
	if err != nil {
		domainerror.WriteProblem(w, r, http.StatusInternalServerError, "internal_error",
			"Internal Server Error", "failed to get trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(apiTrip)
}
