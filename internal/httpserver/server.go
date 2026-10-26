package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	api "github.com/cench-dev/avito-labs/internal/generated"
	tripCreate "github.com/cench-dev/avito-labs/internal/http/trip/create"
	tripFinish "github.com/cench-dev/avito-labs/internal/http/trip/finish"
	tripGet "github.com/cench-dev/avito-labs/internal/http/trip/get"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	httpServer      *http.Server
	r               *chi.Mux
	shutdownTimeout time.Duration
	pool            *pgxpool.Pool
	api.Unimplemented
	createTripHandler *tripCreate.Handler
	getTripHandler    *tripGet.Handler
	finishTripHandler *tripFinish.Handler
}

func NewServer(httpAddress string, shutdownTimeout time.Duration,
	readTimeout time.Duration, readHeaderTimeout time.Duration,
	writeTimeout time.Duration, idleTimeout time.Duration,
	pool *pgxpool.Pool, createTripHandler *tripCreate.Handler,
	getTripHandler *tripGet.Handler,
	finishTripHandler *tripFinish.Handler) *Server {
	r := chi.NewRouter()
	server := &Server{
		r:                 r,
		shutdownTimeout:   shutdownTimeout,
		pool:              pool,
		createTripHandler: createTripHandler,
		getTripHandler:    getTripHandler,
		finishTripHandler: finishTripHandler,
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

	_ = json.NewEncoder(w).Encode(data)
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
		_ = json.NewEncoder(w).Encode(data)
		return
	}
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	s.createTripHandler.Handle(w, r, params)
}

func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.getTripHandler.Handle(w, r, tripId)
}

func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.finishTripHandler.Handle(w, r, tripId)
}
