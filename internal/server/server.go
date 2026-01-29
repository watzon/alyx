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
	"github.com/watzon/alyx/internal/server/requestlog"
	"github.com/watzon/alyx/internal/storage"
)

type Server struct {
	cfg             *config.Config
	db              *database.DB
	schema          *schema.Schema
	schemaPath      string
	configPath      string
	rules           *rules.Engine
	broker          *realtime.Broker
	funcService     *functions.Service
	dbHookTrigger   *DatabaseHookTrigger
	deployService   *deploy.Service
	requestLogs     *requestlog.Store
	httpServer      *http.Server
	router          *Router
	storageService  *storage.Service
	tusService      *storage.TUSService
	signedService   *storage.SignedURLService
	cleanupService  *storage.CleanupService
	loginLimiter    *RateLimiter
	registerLimiter *RateLimiter
	mu              sync.RWMutex
}

const defaultRequestLogCapacity = 1000

type Option func(*Server)

func WithSchemaPath(path string) Option {
	return func(s *Server) {
		s.schemaPath = path
	}
}

func WithConfigPath(path string) Option {
	return func(s *Server) {
		s.configPath = path
	}
}

func New(cfg *config.Config, db *database.DB, s *schema.Schema, opts ...Option) *Server {
	srv := &Server{
		cfg:         cfg,
		db:          db,
		schema:      s,
		schemaPath:  "schema.yaml",
		configPath:  "",
		requestLogs: requestlog.NewStore(defaultRequestLogCapacity),
	}

	for _, opt := range opts {
		opt(srv)
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
			DevMode:      cfg.Dev.Enabled,
			Schema:       s,
			Registrar:    nil,
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

	// Initialize storage service if buckets are defined in schema
	if len(s.Buckets) > 0 && len(cfg.Storage.Backends) > 0 {
		backends := make(map[string]storage.Backend)

		for name, backendCfg := range cfg.Storage.Backends {
			var backend storage.Backend
			var err error

			switch backendCfg.Type {
			case "filesystem":
				if backendCfg.Filesystem == nil {
					log.Warn().Str("backend", name).Msg("Filesystem backend config missing, skipping")
					continue
				}
				if backendCfg.Filesystem.Path == "" {
					log.Warn().Str("backend", name).Msg("Filesystem backend path is required, skipping")
					continue
				}
				if backendCfg.Filesystem.BasePath != "" {
					backend = storage.NewFilesystemBackendWithPrefix(backendCfg.Filesystem.Path, backendCfg.Filesystem.BasePath)
				} else {
					backend = storage.NewFilesystemBackend(backendCfg.Filesystem.Path)
				}

			case "s3":
				if backendCfg.S3 == nil {
					log.Warn().Str("backend", name).Msg("S3 backend config missing, skipping")
					continue
				}
				backend, err = storage.NewS3Backend(context.Background(), *backendCfg.S3)

			default:
				log.Warn().Str("backend", name).Str("type", backendCfg.Type).Msg("Unknown backend type, skipping")
				continue
			}

			if err != nil {
				log.Warn().Err(err).Str("backend", name).Msg("Failed to create backend")
				continue
			}

			backends[name] = backend
		}

		if len(backends) > 0 {
			srv.storageService = storage.NewService(db, backends, s, cfg, rulesEngine)
			srv.tusService = storage.NewTUSService(db, backends, s, cfg, "./tmp")
			srv.signedService = storage.NewSignedURLService([]byte(cfg.Auth.JWT.Secret))
			srv.cleanupService = storage.NewCleanupService(storage.NewTUSStore(db), "./tmp", 1*time.Hour)
		}
	}

	srv.loginLimiter = NewRateLimiter(cfg.Auth.RateLimit.Login)
	srv.registerLimiter = NewRateLimiter(cfg.Auth.RateLimit.Register)

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

		s.dbHookTrigger = NewDatabaseHookTrigger(s.funcService)
		s.router.SetHookTrigger(s.dbHookTrigger)
	}

	if s.cleanupService != nil {
		s.cleanupService.Start(ctx)
		log.Info().Msg("Storage cleanup service started")
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

	if s.cleanupService != nil {
		s.cleanupService.Stop()
		log.Info().Msg("Storage cleanup service stopped")
	}

	if s.loginLimiter != nil {
		s.loginLimiter.Stop()
	}
	if s.registerLimiter != nil {
		s.registerLimiter.Stop()
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

func (s *Server) StorageService() *storage.Service {
	return s.storageService
}

func (s *Server) TUSService() *storage.TUSService {
	return s.tusService
}

func (s *Server) SignedService() *storage.SignedURLService {
	return s.signedService
}

func (s *Server) CleanupService() *storage.CleanupService {
	return s.cleanupService
}

func (s *Server) SchemaPath() string {
	return s.schemaPath
}

func (s *Server) ConfigPath() string {
	return s.configPath
}

func (s *Server) GetCollection(name string) (*database.Collection, error) {
	s.mu.RLock()
	col, ok := s.schema.Collections[name]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("collection %q not found", name)
	}
	coll := database.NewCollection(s.db, col)
	if s.dbHookTrigger != nil {
		coll.SetHookTrigger(s.dbHookTrigger)
	}
	return coll, nil
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

	if s.dbHookTrigger != nil {
		s.dbHookTrigger.Reload()
	}

	log.Info().Int("count", len(s.funcService.ListFunctions())).Msg("Functions reloaded")
	return nil
}

func (s *Server) DeployService() *deploy.Service {
	return s.deployService
}

func (s *Server) RequestLogs() *requestlog.Store {
	return s.requestLogs
}

func (s *Server) LoginLimiter() *RateLimiter {
	return s.loginLimiter
}

func (s *Server) RegisterLimiter() *RateLimiter {
	return s.registerLimiter
}
