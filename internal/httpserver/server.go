package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/cench-dev/avito-labs/internal/http/domainerror"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	api "github.com/cench-dev/avito-labs/internal/generated"
	tripCreate "github.com/cench-dev/avito-labs/internal/http/trip/create"
	tripFinish "github.com/cench-dev/avito-labs/internal/http/trip/finish"
	tripGet "github.com/cench-dev/avito-labs/internal/http/trip/get"
)

type Server struct {
	httpServer      *http.Server
	shutdownTimeout time.Duration
	pool            *pgxpool.Pool
	api.Unimplemented
	createTripHandler *tripCreate.Handler
	getTripHandler    *tripGet.Handler
	finishTripHandler *tripFinish.Handler
}

func NewServer(
	httpAddress string,
	shutdownTimeout time.Duration,
	readTimeout time.Duration,
	readHeaderTimeout time.Duration,
	writeTimeout time.Duration,
	idleTimeout time.Duration,
	pool *pgxpool.Pool,
	createTripHandler *tripCreate.Handler,
	getTripHandler *tripGet.Handler,
	finishTripHandler *tripFinish.Handler,
	requestMetrics RequestMetricsRecorder,
	logger *slog.Logger,
) *Server {
	r := chi.NewRouter()

	r.Use(Recover(logger))
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			route := routePattern(req)
			if route == "" {
				route = "unmatched"
			}

			span := trace.SpanFromContext(req.Context())
			if span.IsRecording() {
				span.SetName(req.Method + " " + route)
			}

			next.ServeHTTP(w, req)
		}), "http.request")
	})
	r.Use(RequestLogging(logger))
	r.Use(RequestMetrics(requestMetrics))

	server := &Server{
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

	api.HandlerWithOptions(server, api.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			domainerror.WriteProblem(
				w,
				r,
				http.StatusBadRequest,
				"invalid_request",
				"Bad Request",
				"invalid request",
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

	_ = json.NewEncoder(w).Encode(api.HealthResponse{
		Status: api.Ok,
	})
}

func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	err := s.pool.Ping(r.Context())

	data := api.HealthResponse{Status: api.Ok}
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
	s.createTripHandler.Handle(w, r)
}

func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.getTripHandler.Handle(w, r, tripId)
}

func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.finishTripHandler.Handle(w, r, tripId)
}
