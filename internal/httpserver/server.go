package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	api "github.com/cench-dev/avito-labs/internal/generated"
	"github.com/cench-dev/avito-labs/internal/repository"
	"github.com/cench-dev/avito-labs/internal/txmanager"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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
	api.HandlerFromMux(server, r)

	return server
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) WaitForShutdown() error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	<-sigs
	err := s.Shutdown()
	if err != nil {
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
	var data api.TripData

	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Bad Request", "invalid request body")
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

	tripID := uuid.New()
	startedAt := time.Now()
	tripParams := repository.CreateTripParams{
		ID:             pgtype.UUID{Bytes: tripID, Valid: true},
		UserID:         pgtype.UUID{Bytes: data.UserId, Valid: true},
		DriverID:       pgtype.UUID{Bytes: data.DriverId, Valid: true},
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "trips_limitation_for_driver_idx" {
			writeProblem(w, r, http.StatusConflict, "driver_busy", "Conflict", "driver already has an active trip")
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
	id := pgtype.UUID{
		Bytes: tripId,
		Valid: true,
	}

	trip, err := s.tripRepository.GetTrip(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
	id := pgtype.UUID{
		Bytes: tripId,
		Valid: true,
	}

	finishedAt := time.Now()
	err := s.txManager.Do(r.Context(), func(ctx context.Context) error {
		if err := s.tripRepository.FinishTrip(ctx, id, finishedAt); err != nil {
			return err
		}

		fromStatus := "active"

		return s.tripRepository.CreateTripStatusHistory(
			ctx,
			repository.CreateTripStatusHistoryParams{
				TripID:     id,
				FromStatus: &fromStatus,
				ToStatus:   "completed",
			},
		)
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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

	trip, err := s.tripRepository.GetTrip(r.Context(), id)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "failed to get finished trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(trip)
}
