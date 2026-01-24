// Package adminui provides the embedded admin UI handler.
package adminui

import (
	"bytes"
	"embed"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/config"
)

//go:embed all:dist
var distFS embed.FS

type Handler struct {
	cfg      *config.AdminUIConfig
	devProxy *httputil.ReverseProxy
	fileFS   http.Handler
}

func New(cfg *config.AdminUIConfig) *Handler {
	h := &Handler{
		cfg: cfg,
	}

	subFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Error().Err(err).Msg("Failed to create sub filesystem for admin UI")
		return h
	}
	h.fileFS = http.FileServer(http.FS(subFS))

	if proxyURL := os.Getenv("ALYX_ADMIN_UI_DEV"); proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			log.Warn().Err(err).Str("url", proxyURL).Msg("Invalid ALYX_ADMIN_UI_DEV URL, using embedded assets")
		} else {
			h.devProxy = httputil.NewSingleHostReverseProxy(parsed)
			// Rewrite the request path to include the base path that Vite expects.
			// The router strips /_admin before we receive the request, but Vite dev
			// server expects paths starting with /_admin due to SvelteKit's base config.
			originalDirector := h.devProxy.Director
			h.devProxy.Director = func(req *http.Request) {
				originalDirector(req)
				// Prepend the base path that was stripped by the router
				req.URL.Path = cfg.Path + req.URL.Path
				if req.URL.RawPath != "" {
					req.URL.RawPath = cfg.Path + req.URL.RawPath
				}
			}
			h.devProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
				log.Debug().Err(err).Msg("Dev proxy unavailable, serving embedded assets")
				h.serveEmbedded(w, r)
			}
			log.Info().Str("proxy", proxyURL).Msg("Admin UI proxying to dev server (ALYX_ADMIN_UI_DEV)")
		}
	}

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.devProxy != nil {
		h.devProxy.ServeHTTP(w, r)
		return
	}

	h.serveEmbedded(w, r)
}

func (h *Handler) serveEmbedded(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "" || path == "/" {
		h.serveFile(w, r, "index.html")
		return
	}

	filePath := strings.TrimPrefix(path, "/")

	subFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if _, err := fs.Stat(subFS, filePath); err == nil {
		h.serveFile(w, r, filePath)
		return
	}

	if !hasAssetExtension(path) {
		h.serveFile(w, r, "index.html")
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	subFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	f, err := subFS.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	content, ok := f.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		content = bytes.NewReader(data)
	}

	http.ServeContent(w, r, name, stat.ModTime(), content)
}

func hasAssetExtension(path string) bool {
	extensions := []string{
		".js", ".css", ".map", ".json",
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp",
		".woff", ".woff2", ".ttf", ".eot",
		".mp4", ".webm", ".ogg", ".mp3", ".wav",
	}
	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}
