package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/deploy"
	"github.com/watzon/alyx/internal/functions"
	"github.com/watzon/alyx/internal/realtime"
	"github.com/watzon/alyx/internal/rules"
	"github.com/watzon/alyx/internal/schema"
)

type Server struct {
	cfg           *config.Config
	db            *database.DB
	schema        *schema.Schema
	rules         *rules.Engine
	broker        *realtime.Broker
	funcService   *functions.Service
	deployService *deploy.Service
	httpServer    *http.Server
	router        *Router
	mu            sync.RWMutex
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

	if cfg.Functions.Enabled {
		funcService, err := functions.NewService(&functions.ServiceConfig{
			FunctionsDir: cfg.Functions.Path,
			Config:       &cfg.Functions,
			ServerPort:   cfg.Server.Port,
		})
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create function service")
		} else {
			srv.funcService = funcService
		}
	}

	schemaPath := "schema.yaml"
	deployService := deploy.NewService(db.DB, schemaPath, cfg.Functions.Path, "migrations")
	if err := deployService.Init(); err != nil {
		log.Warn().Err(err).Msg("Failed to initialize deploy service")
	} else {
		srv.deployService = deployService
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

	if s.funcService != nil {
		if err := s.funcService.Start(ctx); err != nil {
			return fmt.Errorf("starting function service: %w", err)
		}
		log.Info().Int("count", len(s.funcService.ListFunctions())).Msg("Function service started")
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

	if s.funcService != nil {
		if err := s.funcService.Close(); err != nil {
			log.Warn().Err(err).Msg("Error closing function service")
		}
		log.Info().Msg("Function service stopped")
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

func (s *Server) FuncService() *functions.Service {
	return s.funcService
}

func (s *Server) GetCollection(name string) (*database.Collection, error) {
	s.mu.RLock()
	col, ok := s.schema.Collections[name]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("collection %q not found", name)
	}
	return database.NewCollection(s.db, col), nil
}

// UpdateSchema replaces the server's schema and reloads dependent components.
func (s *Server) UpdateSchema(newSchema *schema.Schema) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.schema = newSchema

	if s.rules != nil {
		if err := s.rules.LoadSchema(newSchema); err != nil {
			log.Warn().Err(err).Msg("Failed to reload schema rules")
		}
	}

	if s.broker != nil {
		s.broker.UpdateSchema(newSchema)
	}

	return nil
}

// ReloadFunctions triggers rediscovery of serverless functions.
func (s *Server) ReloadFunctions() error {
	if s.funcService == nil {
		return nil
	}

	if err := s.funcService.ReloadFunctions(); err != nil {
		return fmt.Errorf("reloading functions: %w", err)
	}

	log.Info().Int("count", len(s.funcService.ListFunctions())).Msg("Functions reloaded")
	return nil
}

func (s *Server) DeployService() *deploy.Service {
	return s.deployService
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
