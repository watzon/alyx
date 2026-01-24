package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/realtime"
	"github.com/watzon/alyx/internal/rules"
	"github.com/watzon/alyx/internal/schema"
)

type Server struct {
	cfg        *config.Config
	db         *database.DB
	schema     *schema.Schema
	rules      *rules.Engine
	broker     *realtime.Broker
	httpServer *http.Server
	router     *Router
}

func New(cfg *config.Config, db *database.DB, s *schema.Schema) *Server {
	srv := &Server{
		cfg:    cfg,
		db:     db,
		schema: s,
	}

	rulesEngine, err := rules.NewEngine()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create rules engine, access control disabled")
	} else if err := rulesEngine.LoadSchema(s); err != nil {
		log.Warn().Err(err).Msg("Failed to load schema rules, access control disabled")
		rulesEngine = nil
	}
	srv.rules = rulesEngine

	if cfg.Realtime.Enabled {
		brokerCfg := &realtime.BrokerConfig{
			PollInterval:   cfg.Realtime.PollInterval.Milliseconds(),
			MaxConnections: cfg.Realtime.MaxConnections,
			BufferSize:     cfg.Realtime.ChangeBufferSize,
		}
		srv.broker = realtime.NewBroker(db, s, rulesEngine, brokerCfg)
	}

	srv.router = NewRouter(srv)
	srv.httpServer = &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      srv.router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return srv
}

func (s *Server) Start(ctx context.Context) error {
	log.Info().
		Str("addr", s.cfg.Server.Address()).
		Msg("Starting server")

	if s.broker != nil {
		if err := s.broker.Start(ctx); err != nil {
			return fmt.Errorf("starting realtime broker: %w", err)
		}
		log.Info().Msg("Realtime broker started")
	}

	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Info().Msg("Shutting down server")

	if s.broker != nil {
		s.broker.Stop()
		log.Info().Msg("Realtime broker stopped")
	}

	return s.httpServer.Shutdown(ctx)
}

func (s *Server) DB() *database.DB {
	return s.db
}

func (s *Server) Schema() *schema.Schema {
	return s.schema
}

func (s *Server) Config() *config.Config {
	return s.cfg
}

func (s *Server) Broker() *realtime.Broker {
	return s.broker
}

func (s *Server) Rules() *rules.Engine {
	return s.rules
}

func (s *Server) GetCollection(name string) (*database.Collection, error) {
	col, ok := s.schema.Collections[name]
	if !ok {
		return nil, fmt.Errorf("collection %q not found", name)
	}
	return database.NewCollection(s.db, col), nil
}

type contextKey string

const (
	requestIDKey   contextKey = "request_id"
	requestTimeKey contextKey = "request_time"
)

func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func RequestTime(ctx context.Context) time.Time {
	if t, ok := ctx.Value(requestTimeKey).(time.Time); ok {
		return t
	}
	return time.Time{}
}
