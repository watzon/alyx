package server

import (
	"net/http"
	"strings"

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/server/handlers"
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
	r.Use(LoggingMiddleware)

	if r.server.cfg.Server.CORS.Enabled {
		r.Use(CORSMiddleware(r.server.cfg.Server.CORS))
	}
}

func (r *Router) Use(mw Middleware) {
	r.middlewares = append(r.middlewares, mw)
}

func (r *Router) setupRoutes() {
	h := handlers.New(r.server.DB(), r.server.Schema(), r.server.Config(), r.server.Rules())

	r.mux.HandleFunc("GET /", r.wrap(h.HealthCheck))
	r.mux.HandleFunc("GET /health", r.wrap(h.HealthCheck))

	r.mux.HandleFunc("GET /api/collections/{collection}", r.wrap(h.ListDocuments))
	r.mux.HandleFunc("POST /api/collections/{collection}", r.wrap(h.CreateDocument))
	r.mux.HandleFunc("GET /api/collections/{collection}/{id}", r.wrap(h.GetDocument))
	r.mux.HandleFunc("PATCH /api/collections/{collection}/{id}", r.wrap(h.UpdateDocument))
	r.mux.HandleFunc("PUT /api/collections/{collection}/{id}", r.wrap(h.UpdateDocument))
	r.mux.HandleFunc("DELETE /api/collections/{collection}/{id}", r.wrap(h.DeleteDocument))

	authHandlers := handlers.NewAuthHandlers(r.server.DB(), &r.server.cfg.Auth)
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
