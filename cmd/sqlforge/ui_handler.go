package main

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/drover-org/drover-sqlforge/internal/api"
	"github.com/drover-org/drover-sqlforge/internal/graph"
	"github.com/drover-org/drover-sqlforge/internal/virtual"
)

// newUIHandler wires the Web GUI routes. When bound to a loopback interface
// the handler also rejects non-loopback Host headers (DNS rebinding).
func newUIHandler(dag *graph.DAG, runner virtual.Runner, schema string, dist fs.FS, bindHost string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/dag", api.ServeDAG(dag))
	preview := api.ServeModelPreview(runner, schema, dag)
	details := api.ServeModelDetails(dag)
	mux.HandleFunc("/api/models/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/preview") {
			preview(w, r)
			return
		}
		details(w, r)
	})
	mux.Handle("/", http.FileServer(http.FS(dist)))

	if api.IsLoopbackHost(bindHost) {
		return api.LoopbackHostGuard(mux)
	}
	return mux
}
