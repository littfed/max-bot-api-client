package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/littfed/max-bot-api-client/web"
)

type gzipResponseWriter struct {
	http.ResponseWriter
	writer     io.Writer
	headerSent bool
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.headerSent {
		g.Header().Del("Content-Length")
		g.headerSent = true
	}
	return g.writer.Write(b)
}

func (g *gzipResponseWriter) WriteHeader(statusCode int) {
	g.Header().Del("Content-Length")
	g.headerSent = true
	g.ResponseWriter.WriteHeader(statusCode)
}

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

	serveSPA := func(w http.ResponseWriter, r *http.Request) {
		// Do not intercept API, webhook, or metrics routes
		if strings.HasPrefix(r.URL.Path, "/api/") ||
			strings.HasPrefix(r.URL.Path, "/webhook") ||
			strings.HasPrefix(r.URL.Path, "/metrics") {
			http.NotFound(w, r)
			return
		}

		// Enable transparent gzip compression if supported by client
		var outWriter http.ResponseWriter = w
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			gz := gzip.NewWriter(w)
			defer gz.Close()
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")
			outWriter = &gzipResponseWriter{ResponseWriter: w, writer: gz}
		}

		cleanPath := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if cleanPath == "." || cleanPath == "" || cleanPath == "index.html" ||
			cleanPath == "maxapp" || cleanPath == "webapp" ||
			cleanPath == "hub" || cleanPath == "shop" || cleanPath == "reception" ||
			cleanPath == "support" || cleanPath == "chat" || cleanPath == "admin" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			outWriter.WriteHeader(http.StatusOK)
			_, _ = outWriter.Write(indexHTML)
			return
		}

		// Set aggressive caching for content-hashed assets and vendored SDKs
		if strings.HasPrefix(cleanPath, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if cleanPath == "max-web-app.js" || cleanPath == "telegram-web-app.js" {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}

		// Check if file exists in distFS and is a regular file (prevent directory listing)
		f, err := distFS.Open(cleanPath)
		if err == nil {
			stat, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil {
				if stat.IsDir() {
					// Never serve directory listing
					http.NotFound(w, r)
					return
				}
				ext := path.Ext(cleanPath)
				if mimeType := mime.TypeByExtension(ext); mimeType != "" {
					w.Header().Set("Content-Type", mimeType)
				}
				fileServer.ServeHTTP(outWriter, r)
				return
			}
		}

		// Missing static assets or directory requests under /assets should return 404 rather than fallback HTML
		if cleanPath == "assets" || strings.HasPrefix(cleanPath, "assets/") {
			http.NotFound(w, r)
			return
		}

		// Fallback to index.html for SPA client-side routes (e.g. /shop, /reception, /admin, etc.)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		outWriter.WriteHeader(http.StatusOK)
		_, _ = outWriter.Write(indexHTML)
	}

	mux.HandleFunc("/", serveSPA)
	mux.HandleFunc("/maxapp", serveSPA)
	mux.HandleFunc("/maxapp/", serveSPA)
	mux.HandleFunc("/webapp", serveSPA)
	mux.HandleFunc("/webapp/", serveSPA)

	return nil
}
