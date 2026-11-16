package positions

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"time"

	api "github.com/cench-dev/avito-labs/internal/generated"
	"github.com/cench-dev/avito-labs/internal/http/domainerror"
	"github.com/cench-dev/avito-labs/internal/model"
)

type Handler struct {
	useCase PositionUseCase
	worker  PositionCanceler
}

func NewHandler(useCase PositionUseCase, worker PositionCanceler) *Handler {
	return &Handler{
		useCase: useCase,
		worker:  worker,
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	var body api.PositionData

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		domainerror.WriteProblem(
			w, r, http.StatusBadRequest,
			"invalid_request", "Bad Request", "invalid request body",
		)
		return
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		domainerror.WriteProblem(
			w, r, http.StatusBadRequest,
			"invalid_request", "Bad Request", "invalid request body",
		)
		return
	}

	latitude := body.Coordinates.Latitude
	longitude := body.Coordinates.Longitude

	if math.IsNaN(latitude) || math.IsInf(latitude, 0) ||
		math.IsNaN(longitude) || math.IsInf(longitude, 0) ||
		latitude < -90 || latitude > 90 ||
		longitude < -180 || longitude > 180 ||
		body.RecordedAt.IsZero() ||
		body.RecordedAt.After(time.Now().Add(time.Minute)) {
		domainerror.WriteProblem(
			w, r, http.StatusBadRequest,
			"invalid_request", "Bad Request", "invalid position data",
		)
		return
	}

	position := model.TripPosition{
		TripID:     tripID.String(),
		Latitude:   latitude,
		Longitude:  longitude,
		RecordedAt: body.RecordedAt,
	}

	id, err := h.useCase.Create(r.Context(), position)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	h.worker.Cancel(position.TripID)

	response := api.TripPosition{
		Id:     id,
		TripId: tripID,
		Coordinates: api.Coordinates{
			Latitude:  latitude,
			Longitude: longitude,
		},
		RecordedAt: body.RecordedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	positions, err := h.useCase.Get(r.Context(), tripID.String())
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	response := api.TripPositionsResponse{
		Positions: make([]api.TripPosition, 0, len(positions)),
	}

	for _, position := range positions {
		response.Positions = append(response.Positions, api.TripPosition{
			Id:     position.ID,
			TripId: tripID,
			Coordinates: api.Coordinates{
				Latitude:  position.Latitude,
				Longitude: position.Longitude,
			},
			RecordedAt: position.RecordedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
