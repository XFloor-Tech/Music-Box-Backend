package auth

import (
	"context"
	"encoding/json"

	"github.com/aarondl/authboss/v3"
)

type successJSONRenderer struct{}

func newSuccessJSONRenderer() successJSONRenderer {
	return successJSONRenderer{}
}

func (r successJSONRenderer) Load(names ...string) error {
	return nil
}

func (r successJSONRenderer) Render(ctx context.Context, page string, data authboss.HTMLData) ([]byte, string, error) {
	payload := authboss.HTMLData{}
	for key, value := range data {
		if key == "success" || key == "status" {
			continue
		}

		payload[key] = value
	}

	status, ok := data["status"].(string)
	if !ok || status == "" {
		status = authStatusFromData(data)
	}

	success, ok := data["success"].(bool)
	if !ok {
		success = status == "success"
	}

	body, err := json.Marshal(authboss.HTMLData{
		"success": success,
		"status":  status,
		"data":    payload,
	})
	if err != nil {
		return nil, "", err
	}

	return body, "application/json", nil
}

func authStatusFromData(data authboss.HTMLData) string {
	for _, key := range []string{authboss.DataErr, authboss.DataValidation} {
		if value, ok := data[key]; ok && value != nil {
			return "failure"
		}
	}

	return "success"
}
