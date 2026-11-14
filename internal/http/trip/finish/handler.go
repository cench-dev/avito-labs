package finish

import (
	"encoding/json"
	"net/http"
	"time"

	api "github.com/cench-dev/avito-labs/internal/generated"
	"github.com/cench-dev/avito-labs/internal/http/domainerror"
	tripConverter "github.com/cench-dev/avito-labs/internal/http/trip"
)

type Handler struct {
	finisher TripFinisher
	getter   TripGetter
}

func NewHandler(finisher TripFinisher, getter TripGetter) *Handler {
	return &Handler{finisher: finisher, getter: getter}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	id := tripId.String()

	if err := h.finisher.Finish(r.Context(), id, time.Now()); err != nil {
		domainerror.Write(w, r, err)
		return
	}

	trip, err := h.getter.Get(r.Context(), id)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	apiTrip, err := tripConverter.ConvertAPITrip(trip)
	if err != nil {
		domainerror.WriteProblem(w, r, http.StatusInternalServerError, "internal_error",
			"Internal Server Error", "failed to finish trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(apiTrip)
}
