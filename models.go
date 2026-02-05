package main

import "time"

type RegisterRequest struct {
	ID  string `json:"id"`
	SDP string `json:"sdp"`
}

type RegisteredHost struct {
	ID           string    `json:"id"`
	SDP          string    `json:"sdp"`
	RegisteredAt time.Time `json:"registered_at"`
	RemoteAddr   string    `json:"remote_addr"`
	LastSeen     time.Time `json:"last_seen"`
	Answer       string    `json:"answer,omitempty"`
}

type HostListItem struct {
	ID           string    `json:"id"`
	RegisteredAt time.Time `json:"registered_at"`
	RemoteAddr   string    `json:"remote_addr"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
