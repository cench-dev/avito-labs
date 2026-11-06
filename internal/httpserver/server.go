package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	api "github.com/cench-dev/avito-labs/internal/generated"
	"github.com/cench-dev/avito-labs/internal/repository"
	"github.com/cench-dev/avito-labs/internal/txmanager"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type Server struct {
	httpServer      *http.Server
	r               *chi.Mux
	shutdownTimeout time.Duration
	pool            *pgxpool.Pool
	api.Unimplemented
	tripRepository *repository.TripRepository
	txManager      txmanager.TxManager
}

func NewServer(httpAddress string, shutdownTimeout time.Duration,
	readTimeout time.Duration, readHeaderTimeout time.Duration,
	writeTimeout time.Duration, idleTimeout time.Duration, pool *pgxpool.Pool, repo *repository.TripRepository, txManager txmanager.TxManager) *Server {
	r := chi.NewRouter()
	server := &Server{
		r:               r,
		shutdownTimeout: shutdownTimeout,
		pool:            pool,
		tripRepository:  repo,
		txManager:       txManager,
	}
	server.httpServer = &http.Server{
		Addr:              httpAddress,
		Handler:           r,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	api.HandlerWithOptions(server, api.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeProblem(
				w,
				r,
				http.StatusBadRequest,
				"invalid_request",
				"Invalid request",
				err.Error(),
			)
		},
	})

	return server
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		closeErr := s.httpServer.Close()
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}

	return nil
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	data := api.HealthResponse{
		Status: api.Ok,
	}

	json.NewEncoder(w).Encode(data)
}

func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	err := s.pool.Ping(r.Context())
	data := api.HealthResponse{
		Status: api.Ok,
	}
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		data.Status = api.Unavailable
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(data)
		return
	}
	json.NewEncoder(w).Encode(data)
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code string, title string, detail string) {
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

	json.NewEncoder(w).Encode(problem)
}

func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "invalid request body")
		return
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "invalid request body")
		return
	}

	requiredFields := []string{
		"user_id",
		"driver_id",
		"start_point",
		"end_point",
		"price",
	}

	for _, field := range requiredFields {
		value, exists := fields[field]
		if !exists || string(value) == "null" {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "missing required field: "+field)
			return
		}
	}

	var data api.TripData
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&data); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "invalid request body")
		return
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "request body must contain exactly one JSON object")
		return
	}

	if data.UserId == uuid.Nil || data.DriverId == uuid.Nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "user_id and driver_id are required")
		return
	}

	if data.Price < 0 ||
		data.StartPoint.Latitude < -90 || data.StartPoint.Latitude > 90 ||
		data.StartPoint.Longitude < -180 || data.StartPoint.Longitude > 180 ||
		data.EndPoint.Latitude < -90 || data.EndPoint.Latitude > 90 ||
		data.EndPoint.Longitude < -180 || data.EndPoint.Longitude > 180 {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "invalid request body")
		return
	}

	for _, field := range []string{"start_point", "end_point"} {
		var coordinates map[string]json.RawMessage

		if err := json.Unmarshal(fields[field], &coordinates); err != nil {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "invalid coordinates")
			return
		}

		for _, coordinate := range []string{"latitude", "longitude"} {
			value, exists := coordinates[coordinate]
			if !exists || string(value) == "null" {
				writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "missing required coordinate: "+coordinate)
				return
			}
		}
	}

	tripID := uuid.New()
	startedAt := time.Now()
	tripParams := repository.CreateTripParams{
		ID:             tripID,
		UserID:         data.UserId,
		DriverID:       data.DriverId,
		StartLatitude:  data.StartPoint.Latitude,
		StartLongitude: data.StartPoint.Longitude,
		EndLatitude:    data.EndPoint.Latitude,
		EndLongitude:   data.EndPoint.Longitude,
		Price:          data.Price,
		Status:         "active",
		StartedAt:      startedAt,
	}

	err = s.txManager.Do(r.Context(), func(ctx context.Context) error {
		if err := s.tripRepository.CreateTrip(ctx, tripParams); err != nil {
			return err
		}

		return s.tripRepository.CreateTripStatusHistory(ctx, repository.CreateTripStatusHistoryParams{
			TripID:   tripParams.ID,
			ToStatus: "active",
		})
	})

	if err != nil {
		if errors.Is(err, repository.ErrDriverBusy) {
			writeProblem(w, r, http.StatusConflict, "driver_busy", "Driver is busy", "")
			return
		}

		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "failed to create trip")
		return
	}

	trip := api.Trip{
		Id:         openapi_types.UUID(tripID),
		UserId:     data.UserId,
		DriverId:   data.DriverId,
		StartPoint: data.StartPoint,
		EndPoint:   data.EndPoint,
		Price:      data.Price,
		Status:     api.Active,
		StartedAt:  startedAt,
		FinishedAt: nil,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+tripID.String())
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(trip)
}

func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {

	trip, err := s.tripRepository.GetTrip(r.Context(), tripId)
	if err != nil {
		if errors.Is(err, repository.ErrTripNotFound) {
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Not Found", "trip not found")
			return
		}

		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "failed to get trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(trip)
}

func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	finishedAt := time.Now()
	err := s.txManager.Do(r.Context(), func(ctx context.Context) error {
		if err := s.tripRepository.FinishTrip(ctx, tripId, finishedAt); err != nil {
			return err
		}

		fromStatus := "active"

		return s.tripRepository.CreateTripStatusHistory(
			ctx,
			repository.CreateTripStatusHistoryParams{
				TripID:     tripId,
				FromStatus: &fromStatus,
				ToStatus:   "completed",
			},
		)
	})

	if err != nil {
		if errors.Is(err, repository.ErrTripNotFound) {
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Not Found", "trip not found")
			return
		}

		if errors.Is(err, repository.ErrTripAlreadyCompleted) {
			writeProblem(w, r, http.StatusConflict, "trip_completed", "Conflict", "trip already completed")
			return
		}

		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "failed to finish trip")
		return
	}

	trip, err := s.tripRepository.GetTrip(r.Context(), tripId)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "failed to get finished trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(trip)
}
