package handlers

import (
	"fmt"
	"net/http"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/openapi"
	"github.com/watzon/alyx/internal/schema"
)

type DocsHandler struct {
	schema    *schema.Schema
	cfg       *config.Config
	specCache []byte
}

func NewDocsHandler(s *schema.Schema, cfg *config.Config) *DocsHandler {
	return &DocsHandler{
		schema: s,
		cfg:    cfg,
	}
}

func (h *DocsHandler) OpenAPISpec(w http.ResponseWriter, r *http.Request) {
	if h.specCache == nil {
		serverURL := fmt.Sprintf("http://%s", h.cfg.Server.Address())
		if r.TLS != nil {
			serverURL = fmt.Sprintf("https://%s", r.Host)
		} else if fwdProto := r.Header.Get("X-Forwarded-Proto"); fwdProto != "" {
			serverURL = fmt.Sprintf("%s://%s", fwdProto, r.Host)
		}

		spec := openapi.Generate(h.schema, openapi.GeneratorConfig{
			Title:       h.cfg.Docs.Title,
			Description: h.cfg.Docs.Description,
			Version:     h.cfg.Docs.Version,
			ServerURL:   serverURL,
		})

		data, err := spec.JSON()
		if err != nil {
			Error(w, http.StatusInternalServerError, "SPEC_ERROR", "Failed to generate OpenAPI spec")
			return
		}
		h.specCache = data
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.specCache)
}

func (h *DocsHandler) DocsUI(w http.ResponseWriter, r *http.Request) {
	var html string

	switch h.cfg.Docs.UI {
	case "swagger":
		html = swaggerUIHTML(h.cfg.Docs.Title)
	case "redoc":
		html = redocHTML(h.cfg.Docs.Title)
	case "stoplight":
		html = stoplightHTML(h.cfg.Docs.Title)
	case "scalar":
		fallthrough
	default:
		html = scalarHTML(h.cfg.Docs.Title)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

func scalarHTML(title string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <title>%s</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <script id="api-reference" data-url="/api/openapi.json"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`, title)
}

func swaggerUIHTML(title string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <title>%s</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = () => {
      SwaggerUIBundle({
        url: '/api/openapi.json',
        dom_id: '#swagger-ui',
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
        layout: 'StandaloneLayout'
      });
    };
  </script>
</body>
</html>`, title)
}

func redocHTML(title string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <title>%s</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet" />
  <style>body { margin: 0; padding: 0; }</style>
</head>
<body>
  <redoc spec-url='/api/openapi.json'></redoc>
  <script src="https://cdn.jsdelivr.net/npm/redoc@latest/bundles/redoc.standalone.js"></script>
</body>
</html>`, title)
}

func stoplightHTML(title string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <title>%s</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <elements-api
    apiDescriptionUrl="/api/openapi.json"
    router="hash"
    layout="sidebar"
  />
  <script src="https://unpkg.com/@stoplight/elements/web-components.min.js"></script>
  <link rel="stylesheet" href="https://unpkg.com/@stoplight/elements/styles.min.css" />
</body>
</html>`, title)
}
