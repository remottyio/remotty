package main

import (
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

func startServer(port string) error {
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	store := NewMemoryStore()
	handler := NewHandler(store, logger)

	// Start background sweeper to remove inactive hosts
	// Timeout: 15 seconds (10s long poll + 5s grace period)
	go startSweeper(store, 15*time.Second, 10*time.Second, logger)

	mux := http.NewServeMux()

	// Root endpoint - HTML view
	mux.HandleFunc("/", handler.Root)

	// Health endpoint (for load balancers)
	mux.HandleFunc("/health", handler.Health)

	// API v1 endpoints
	mux.HandleFunc("/api/1/register", handler.Register)
	mux.HandleFunc("/api/1/list", handler.List)
	mux.HandleFunc("/api/1/connect/", handler.Connect)
	mux.HandleFunc("/api/1/answer/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.PostAnswer(w, r)
		} else if r.Method == http.MethodGet {
			handler.GetAnswer(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

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

func startSweeper(store Store, timeout time.Duration, interval time.Duration, log *logrus.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.WithFields(logrus.Fields{
		"timeout":  timeout,
		"interval": interval,
	}).Info("Starting inactive host sweeper")

	for range ticker.C {
		removed := store.SweepInactive(timeout)
		if removed > 0 {
			log.WithField("count", removed).Info("Swept inactive hosts")
		}
	}
}
