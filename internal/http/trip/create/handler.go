package create

import (
	"encoding/json"
	"net/http"
	"time"

	tripDomain "github.com/cench-dev/avito-labs/internal/domain/trip"
	api "github.com/cench-dev/avito-labs/internal/generated"
	"github.com/cench-dev/avito-labs/internal/http/domainerror"
	tripConverter "github.com/cench-dev/avito-labs/internal/http/trip"
	"github.com/cench-dev/avito-labs/internal/model"
	"github.com/google/uuid"
)

type Handler struct {
	useCase TripCreator
}

func NewHandler(useCase TripCreator) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var data api.TripData

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		domainerror.WriteProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "invalid request body")
		return
	}

	if data.Price < 0 ||
		data.StartPoint.Latitude < -90 || data.StartPoint.Latitude > 90 ||
		data.StartPoint.Longitude < -180 || data.StartPoint.Longitude > 180 ||
		data.EndPoint.Latitude < -90 || data.EndPoint.Latitude > 90 ||
		data.EndPoint.Longitude < -180 || data.EndPoint.Longitude > 180 {
		domainerror.WriteProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "invalid request body")
		return
	}

	tripID := uuid.New()
	tripParams := model.Trip{
		ID:             tripID.String(),
		UserID:         data.UserId.String(),
		DriverID:       data.DriverId.String(),
		StartLatitude:  data.StartPoint.Latitude,
		StartLongitude: data.StartPoint.Longitude,
		EndLatitude:    data.EndPoint.Latitude,
		EndLongitude:   data.EndPoint.Longitude,
		Price:          data.Price,
		Status:         string(tripDomain.StatusActive),
		StartedAt:      time.Now(),
	}

	if err := h.useCase.Create(r.Context(), tripParams); err != nil {
		domainerror.Write(w, r, err)
		return
	}

	apiTrip, err := tripConverter.ConvertAPITrip(tripParams)
	if err != nil {
		domainerror.WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "failed to create trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+tripID.String())
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(apiTrip)
}
