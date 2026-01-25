package server

import (
	"net/http"
	"strings"

	"github.com/watzon/alyx/internal/adminui"
	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/functions"
	"github.com/watzon/alyx/internal/metrics"
	"github.com/watzon/alyx/internal/server/handlers"
	"github.com/watzon/alyx/internal/server/requestlog"
)

type Router struct {
	server      *Server
	mux         *http.ServeMux
	middlewares []Middleware
}

type Middleware func(http.Handler) http.Handler

func NewRouter(srv *Server) *Router {
	r := &Router{
		server: srv,
		mux:    http.NewServeMux(),
	}

	r.setupMiddleware()
	r.setupRoutes()

	return r
}

func (r *Router) setupMiddleware() {
	r.Use(RecoveryMiddleware)
	r.Use(RequestIDMiddleware)
	r.Use(MetricsMiddleware)
	r.Use(LoggingMiddleware)
	r.Use(requestlog.Middleware(r.server.RequestLogs()))

	if r.server.cfg.Server.CORS.Enabled {
		r.Use(CORSMiddleware(r.server.cfg.Server.CORS))
	}
}

func (r *Router) Use(mw Middleware) {
	r.middlewares = append(r.middlewares, mw)
}

func (r *Router) setupRoutes() {
	h := handlers.New(r.server.DB(), r.server.Schema(), r.server.Config(), r.server.Rules())

	if r.server.cfg.AdminUI.Enabled {
		uiHandler := adminui.New(&r.server.cfg.AdminUI)
		basePath := r.server.cfg.AdminUI.Path
		r.mux.Handle("GET "+basePath+"/{path...}", http.StripPrefix(basePath, uiHandler))
		r.mux.Handle("GET "+basePath, http.RedirectHandler(basePath+"/", http.StatusMovedPermanently))
	}

	healthHandlers := handlers.NewHealthHandlers(
		r.server.DB(),
		r.server.Broker(),
		r.server.FuncService(),
		"0.1.0",
	)
	r.mux.HandleFunc("GET /", r.wrap(healthHandlers.Liveness))
	r.mux.HandleFunc("GET /health", r.wrap(healthHandlers.Health))
	r.mux.HandleFunc("GET /health/live", r.wrap(healthHandlers.Liveness))
	r.mux.HandleFunc("GET /health/ready", r.wrap(healthHandlers.Readiness))
	r.mux.HandleFunc("GET /health/stats", r.wrap(healthHandlers.Stats))
	r.mux.Handle("GET /metrics", metrics.Handler())

	r.mux.HandleFunc("GET /api/config", r.wrap(h.Config))
	r.mux.HandleFunc("GET /api/collections/{collection}", r.wrap(h.ListDocuments))
	r.mux.HandleFunc("POST /api/collections/{collection}", r.wrap(h.CreateDocument))
	r.mux.HandleFunc("GET /api/collections/{collection}/{id}", r.wrap(h.GetDocument))
	r.mux.HandleFunc("PATCH /api/collections/{collection}/{id}", r.wrap(h.UpdateDocument))
	r.mux.HandleFunc("PUT /api/collections/{collection}/{id}", r.wrap(h.UpdateDocument))
	r.mux.HandleFunc("DELETE /api/collections/{collection}/{id}", r.wrap(h.DeleteDocument))

	authHandlers := handlers.NewAuthHandlers(r.server.DB(), &r.server.cfg.Auth)
	r.mux.HandleFunc("GET /api/auth/status", r.wrap(authHandlers.Status))
	r.mux.HandleFunc("POST /api/auth/register", r.wrap(authHandlers.Register))
	r.mux.HandleFunc("POST /api/auth/login", r.wrap(authHandlers.Login))
	r.mux.HandleFunc("POST /api/auth/refresh", r.wrap(authHandlers.Refresh))
	r.mux.HandleFunc("POST /api/auth/logout", r.wrap(authHandlers.Logout))
	r.mux.HandleFunc("GET /api/auth/providers", r.wrap(authHandlers.Providers))
	r.mux.HandleFunc("GET /api/auth/oauth/{provider}", r.wrap(authHandlers.OAuthRedirect))
	r.mux.HandleFunc("GET /api/auth/oauth/{provider}/callback", r.wrap(authHandlers.OAuthCallback))
	r.mux.HandleFunc("GET /api/auth/me", r.wrapWithAuth(authHandlers.Me, authHandlers.Service()))

	if r.server.cfg.Docs.Enabled {
		docs := handlers.NewDocsHandler(r.server.Schema(), r.server.Config())
		r.mux.HandleFunc("GET /api/openapi.json", r.wrap(docs.OpenAPISpec))
		r.mux.HandleFunc("GET /api/docs", r.wrap(docs.DocsUI))
		r.mux.HandleFunc("GET /api/docs/", r.wrap(docs.DocsUI))
	}

	if r.server.cfg.Realtime.Enabled && r.server.Broker() != nil {
		rt := handlers.NewRealtimeHandler(r.server.Broker())
		r.mux.HandleFunc("GET /api/realtime", rt.HandleWebSocket)
	}

	if r.server.cfg.Functions.Enabled && r.server.FuncService() != nil {
		funcHandlers := handlers.NewFunctionHandlers(r.server.FuncService())
		r.mux.HandleFunc("POST /api/functions/{name}", r.wrap(funcHandlers.Invoke))
		r.mux.HandleFunc("GET /api/functions", r.wrap(funcHandlers.List))
		r.mux.HandleFunc("GET /api/functions/stats", r.wrap(funcHandlers.Stats))
		r.mux.HandleFunc("POST /api/functions/reload", r.wrap(funcHandlers.Reload))

		internalHandlers := handlers.NewInternalHandlers(
			r.server.DB(),
			r.server.Schema(),
			r.server.FuncService().TokenStore(),
			r.server.FuncService(),
		)
		r.mux.HandleFunc("POST /internal/v1/db/query", r.wrap(internalHandlers.Query))
		r.mux.HandleFunc("GET /internal/v1/db/query", r.wrap(internalHandlers.QueryGET))
		r.mux.HandleFunc("POST /internal/v1/db/exec", r.wrap(internalHandlers.Exec))
		r.mux.HandleFunc("POST /internal/v1/db/tx", r.wrap(internalHandlers.Transaction))
	}

	logsHandlers := handlers.NewLogsHandlers(r.server.RequestLogs())
	r.mux.HandleFunc("GET /api/admin/logs", r.wrap(logsHandlers.List))
	r.mux.HandleFunc("GET /api/admin/logs/stats", r.wrap(logsHandlers.Stats))
	r.mux.HandleFunc("POST /api/admin/logs/clear", r.wrap(logsHandlers.Clear))
	// TODO: Event system routes will be enabled when components are initialized in server
	// if hookRegistry := r.server.HookRegistry(); hookRegistry != nil {
	// 	hookHandlers := handlers.NewHookHandlers(hookRegistry)
	// 	r.mux.HandleFunc("GET /api/hooks", r.wrap(hookHandlers.List))
	// 	r.mux.HandleFunc("POST /api/hooks", r.wrap(hookHandlers.Create))
	// 	r.mux.HandleFunc("GET /api/hooks/{id}", r.wrap(hookHandlers.Get))
	// 	r.mux.HandleFunc("PATCH /api/hooks/{id}", r.wrap(hookHandlers.Update))
	// 	r.mux.HandleFunc("DELETE /api/hooks/{id}", r.wrap(hookHandlers.Delete))
	// 	r.mux.HandleFunc("GET /api/functions/{name}/hooks", r.wrap(hookHandlers.ListForFunction))
	// }

	// if webhookStore := r.server.WebhookStore(); webhookStore != nil {
	// 	webhookHandlers := handlers.NewWebhookHandlers(webhookStore)
	// 	r.mux.HandleFunc("GET /api/webhooks", r.wrap(webhookHandlers.List))
	// 	r.mux.HandleFunc("POST /api/webhooks", r.wrap(webhookHandlers.Create))
	// 	r.mux.HandleFunc("GET /api/webhooks/{id}", r.wrap(webhookHandlers.Get))
	// 	r.mux.HandleFunc("PATCH /api/webhooks/{id}", r.wrap(webhookHandlers.Update))
	// 	r.mux.HandleFunc("DELETE /api/webhooks/{id}", r.wrap(webhookHandlers.Delete))
	// }

	// if scheduleStore := r.server.ScheduleStore(); scheduleStore != nil {
	// 	if scheduler := r.server.Scheduler(); scheduler != nil {
	// 		scheduleHandlers := handlers.NewScheduleHandlers(scheduleStore, scheduler)
	// 		r.mux.HandleFunc("GET /api/schedules", r.wrap(scheduleHandlers.List))
	// 		r.mux.HandleFunc("POST /api/schedules", r.wrap(scheduleHandlers.Create))
	// 		r.mux.HandleFunc("GET /api/schedules/{id}", r.wrap(scheduleHandlers.Get))
	// 		r.mux.HandleFunc("PATCH /api/schedules/{id}", r.wrap(scheduleHandlers.Update))
	// 		r.mux.HandleFunc("DELETE /api/schedules/{id}", r.wrap(scheduleHandlers.Delete))
	// 		r.mux.HandleFunc("POST /api/schedules/{id}/trigger", r.wrap(scheduleHandlers.Trigger))
	// 	}
	// }

	// if executionStore := r.server.ExecutionStore(); executionStore != nil {
	// 	executionHandlers := handlers.NewExecutionHandlers(executionStore)
	// 	r.mux.HandleFunc("GET /api/executions", r.wrap(executionHandlers.List))
	// 	r.mux.HandleFunc("GET /api/executions/{id}", r.wrap(executionHandlers.Get))
	// 	r.mux.HandleFunc("GET /api/functions/{name}/executions", r.wrap(executionHandlers.ListForFunction))
	// }

	if r.server.DeployService() != nil {
		var funcSvc *functions.Service
		if r.server.FuncService() != nil {
			funcSvc = r.server.FuncService()
		}
		adminHandlers := handlers.NewAdminHandlers(
			r.server.DeployService(),
			authHandlers.Service(),
			r.server.DB(),
			r.server.Schema(),
			funcSvc,
			r.server.Config(),
			r.server.SchemaPath(),
			r.server.ConfigPath(),
		)
		r.mux.HandleFunc("GET /api/admin/stats", r.wrap(adminHandlers.Stats))
		r.mux.HandleFunc("POST /api/admin/deploy/prepare", r.wrap(adminHandlers.DeployPrepare))
		r.mux.HandleFunc("POST /api/admin/deploy/execute", r.wrap(adminHandlers.DeployExecute))
		r.mux.HandleFunc("POST /api/admin/deploy/rollback", r.wrap(adminHandlers.DeployRollback))
		r.mux.HandleFunc("GET /api/admin/deploy/history", r.wrap(adminHandlers.DeployHistory))
		r.mux.HandleFunc("GET /api/admin/schema", r.wrap(adminHandlers.SchemaGet))
		r.mux.HandleFunc("GET /api/admin/schema/raw", r.wrap(adminHandlers.SchemaRawGet))
		r.mux.HandleFunc("PUT /api/admin/schema/raw", r.wrap(adminHandlers.SchemaRawUpdate))
		r.mux.HandleFunc("POST /api/admin/schema/validate-rule", r.wrap(adminHandlers.ValidateRule))
		r.mux.HandleFunc("GET /api/admin/config/raw", r.wrap(adminHandlers.ConfigRawGet))
		r.mux.HandleFunc("PUT /api/admin/config/raw", r.wrap(adminHandlers.ConfigRawUpdate))
		r.mux.HandleFunc("POST /api/admin/tokens", r.wrap(adminHandlers.TokenCreate))
		r.mux.HandleFunc("GET /api/admin/tokens", r.wrap(adminHandlers.TokenList))
		r.mux.HandleFunc("DELETE /api/admin/tokens/{name}", r.wrap(adminHandlers.TokenDelete))

		r.mux.HandleFunc("GET /api/admin/users", r.wrap(adminHandlers.UserList))
		r.mux.HandleFunc("POST /api/admin/users", r.wrap(adminHandlers.UserCreate))
		r.mux.HandleFunc("GET /api/admin/users/{id}", r.wrap(adminHandlers.UserGet))
		r.mux.HandleFunc("PATCH /api/admin/users/{id}", r.wrap(adminHandlers.UserUpdate))
		r.mux.HandleFunc("DELETE /api/admin/users/{id}", r.wrap(adminHandlers.UserDelete))
		r.mux.HandleFunc("POST /api/admin/users/{id}/password", r.wrap(adminHandlers.UserSetPassword))
	}
}

func (r *Router) wrap(fn handlers.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		fn(w, req)
	}
}

func (r *Router) wrapWithAuth(fn handlers.HandlerFunc, authService *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		middleware := auth.RequireAuth(authService)
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			fn(w, req)
		}))
		handler.ServeHTTP(w, req)
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	handler := http.Handler(r.mux)

	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i](handler)
	}

	handler.ServeHTTP(w, req)
}

func PathParam(r *http.Request, name string) string {
	return r.PathValue(name)
}

func QueryParams(r *http.Request, name string) []string {
	return r.URL.Query()[name]
}

func QueryParam(r *http.Request, name string) string {
	return r.URL.Query().Get(name)
}

func ParseFilters(r *http.Request) []string {
	return QueryParams(r, "filter")
}

func ParseSorts(r *http.Request) []string {
	sortParam := QueryParam(r, "sort")
	if sortParam == "" {
		return nil
	}
	return strings.Split(sortParam, ",")
}

func ParseExpand(r *http.Request) []string {
	expandParam := QueryParam(r, "expand")
	if expandParam == "" {
		return nil
	}
	return strings.Split(expandParam, ",")
}
