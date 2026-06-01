package server

import "net/http"

type healthResponse struct {
	Status  string `json:"status" example:"ok"`
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
	w.Write([]byte(`{"status":"ok","service":"music-player"}`))
}
