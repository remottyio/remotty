package main

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

var templates = template.Must(template.ParseGlob("templates/*.html"))

type Handler struct {
	store Store
	log   *logrus.Logger
}

func NewHandler(store Store, log *logrus.Logger) *Handler {
	return &Handler{
		store: store,
		log:   log,
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.WithError(err).Warn("Invalid JSON in register request")
		h.respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.ID == "" {
		h.respondError(w, http.StatusBadRequest, "ID is required")
		return
	}
	if req.SDP == "" {
		h.respondError(w, http.StatusBadRequest, "SDP is required")
		return
	}

	remoteAddr := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		remoteAddr = strings.Split(xff, ",")[0]
	}

	host := &RegisteredHost{
		ID:         req.ID,
		SDP:        req.SDP,
		RemoteAddr: remoteAddr,
	}

	if err := h.store.Register(host); err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			h.log.WithField("id", req.ID).Warn("Duplicate ID registration attempt")
			h.respondError(w, http.StatusConflict, "ID already registered")
			return
		}
		h.log.WithError(err).Error("Failed to register host")
		h.respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	h.log.WithFields(logrus.Fields{
		"id":     req.ID,
		"remote": remoteAddr,
	}).Info("Host registered successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "registered",
		"id":     req.ID,
	})
}

func (h *Handler) Root(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	hosts, err := h.store.List()
	if err != nil {
		h.log.WithError(err).Error("Failed to list hosts")
		h.respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	h.renderHostListHTML(w, hosts)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	hosts, err := h.store.List()
	if err != nil {
		h.log.WithError(err).Error("Failed to list hosts")
		h.respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hosts)
}

func (h *Handler) Connect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/1/connect/")
	id := strings.TrimSpace(path)

	if id == "" {
		h.respondError(w, http.StatusBadRequest, "ID is required")
		return
	}

	host, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "Host not found")
			return
		}
		h.log.WithError(err).Error("Failed to get host")
		h.respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Check if browser wants raw SDP for WebRTC
	wantRaw := r.URL.Query().Get("raw") == "true"
	if wantRaw {
		rawSDP, err := DecodeSDP(host.SDP)
		if err != nil {
			h.log.WithError(err).Error("Failed to decode SDP")
			h.respondError(w, http.StatusInternalServerError, "Failed to decode SDP")
			return
		}
		h.log.WithField("id", id).Info("Raw SDP retrieved for WebRTC connection")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":  host.ID,
			"sdp": rawSDP,
		})
		return
	}

	h.log.WithField("id", id).Info("SDP retrieved for connection")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":  host.ID,
		"sdp": host.SDP,
	})
}

func (h *Handler) PostAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/1/answer/")
	id := strings.TrimSpace(path)

	if id == "" {
		h.respondError(w, http.StatusBadRequest, "ID is required")
		return
	}

	var req struct {
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.WithError(err).Warn("Invalid JSON in answer request")
		h.respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Answer == "" {
		h.respondError(w, http.StatusBadRequest, "Answer is required")
		return
	}

	// Encode the raw SDP answer before storing
	encodedAnswer, err := EncodeSDP(req.Answer)
	if err != nil {
		h.log.WithError(err).Error("Failed to encode answer")
		h.respondError(w, http.StatusInternalServerError, "Failed to encode answer")
		return
	}

	if err := h.store.SetAnswer(id, encodedAnswer); err != nil {
		if errors.Is(err, ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "Host not found")
			return
		}
		h.log.WithError(err).Error("Failed to set answer")
		h.respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	h.log.WithField("id", id).Info("Answer received for host")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "answer_received",
		"id":     id,
	})
}

func (h *Handler) GetAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/1/answer/")
	id := strings.TrimSpace(path)

	if id == "" {
		h.respondError(w, http.StatusBadRequest, "ID is required")
		return
	}

	host, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.respondError(w, http.StatusNotFound, "Host not found")
			return
		}
		h.log.WithError(err).Error("Failed to get host")
		h.respondError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	if host.Answer == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.log.WithField("id", id).Info("Answer retrieved by host")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":     host.ID,
		"answer": host.Answer,
	})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (h *Handler) respondError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func (h *Handler) renderHostListHTML(w http.ResponseWriter, hosts []*HostListItem) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := struct {
		Hosts []*HostListItem
	}{
		Hosts: hosts,
	}

	if err := templates.ExecuteTemplate(w, "list.html", data); err != nil {
		h.log.WithError(err).Error("Failed to render template")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
