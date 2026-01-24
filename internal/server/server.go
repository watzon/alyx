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
	"github.com/watzon/alyx/internal/schema"
)

type Server struct {
	cfg        *config.Config
	db         *database.DB
	schema     *schema.Schema
	httpServer *http.Server
	router     *Router
}

func New(cfg *config.Config, db *database.DB, s *schema.Schema) *Server {
	srv := &Server{
		cfg:    cfg,
		db:     db,
		schema: s,
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

func (s *Server) Start() error {
	log.Info().
		Str("addr", s.cfg.Server.Address()).
		Msg("Starting server")

	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Info().Msg("Shutting down server")
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
