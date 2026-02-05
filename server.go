package main

import (
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

func startServer(port string) error {
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	store := NewMemoryStore()
	handler := NewHandler(store, logger)

	mux := http.NewServeMux()

	// Root endpoint - HTML view
	mux.HandleFunc("/", handler.Root)

	// API endpoints
	mux.HandleFunc("/register", handler.Register)
	mux.HandleFunc("/list", handler.List)
	mux.HandleFunc("/connect/", handler.Connect)
	mux.HandleFunc("/answer/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.PostAnswer(w, r)
		} else if r.Method == http.MethodGet {
			handler.GetAnswer(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/health", handler.Health)

	// Static files
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	logger.WithField("port", port).Info("Starting remotty server")

	return http.ListenAndServe(addr, loggingMiddleware(mux))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.WithFields(logrus.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
			"remote": r.RemoteAddr,
		}).Info("Request received")
		next.ServeHTTP(w, r)
	})
}
