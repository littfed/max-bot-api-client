package main

import (
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/littfed/max-bot-api-client/web"
)

// RegisterWebSPA mounts the embedded React 18 SPA static file server and client-side routing fallback.
func RegisterWebSPA(mux *http.ServeMux) error {
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("web_embed: failed to create sub FS for dist: %w", err)
	}

	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return fmt.Errorf("web_embed: failed to read index.html: %w", err)
	}

	fileServer := http.FileServer(http.FS(distFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Do not intercept API, webhook, or metrics routes
		if strings.HasPrefix(r.URL.Path, "/api/") ||
			strings.HasPrefix(r.URL.Path, "/webhook") ||
			strings.HasPrefix(r.URL.Path, "/metrics") {
			http.NotFound(w, r)
			return
		}

		cleanPath := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if cleanPath == "." || cleanPath == "" || cleanPath == "index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexHTML)
			return
		}

		// Check if file exists in distFS
		f, err := distFS.Open(cleanPath)
		if err == nil {
			_ = f.Close()
			// Set proper content type based on extension
			ext := filepath.Ext(cleanPath)
			if mimeType := mime.TypeByExtension(ext); mimeType != "" {
				w.Header().Set("Content-Type", mimeType)
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// Missing static assets under /assets/ should return 404 rather than fallback HTML
		if strings.HasPrefix(cleanPath, "assets/") {
			http.NotFound(w, r)
			return
		}

		// Fallback to index.html for SPA client-side routes (e.g. /shop, /reception, /admin, etc.)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(indexHTML)
	})

	return nil
}
