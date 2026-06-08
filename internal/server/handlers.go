package server

import (
	"encoding/json"
	"net/http"
)

type healthResponse struct {
	Success bool               `json:"success" example:"true"`
	Status  string             `json:"status" example:"ok"`
	Data    healthResponseData `json:"data"`
}

type healthResponseData struct {
	Service string `json:"service" example:"music-player"`
}

// healthCheck godoc
// @Summary Health check
// @Description Returns service health status.
// @Tags health
// @Produce json
// @Success 200 {object} healthResponse
// @Router /health [get]
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthResponse{
		Success: true,
		Status:  "ok",
		Data: healthResponseData{
			Service: "music-player",
		},
	})
}
