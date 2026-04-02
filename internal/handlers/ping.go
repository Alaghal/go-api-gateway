package handlers

import (
	"net/http"
)

type PingResponse struct {
	Message string `json:"message"`
}

func Ping() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, PingResponse{
			Message: "pong",
		})
	}
}
