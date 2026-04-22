package docs

import (
	_ "embed"
	"fmt"
	"net/http"
	"text/template"

	"github.com/gorilla/mux"
)

//go:embed openapi.yaml
var openapiSpec []byte

var uiTemplate = template.Must(template.New("scalar").Parse(`<!doctype html>
<html>
  <head>
    <title>Gym API — Reference</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <script
      id="api-reference"
      data-url="/docs/openapi.yaml"
      data-configuration='{"servers":[{"url":"{{.}}","description":"Current"}]}'
    ></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`))

func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/docs", serveUI).Methods(http.MethodGet)
	r.HandleFunc("/docs/openapi.yaml", serveSpec).Methods(http.MethodGet)
}

func serveUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	uiTemplate.Execute(w, serverURL(r))
}

func serveSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(openapiSpec)
}

func serverURL(r *http.Request) string {
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}
