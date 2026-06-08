package auth

import (
	"context"
	"encoding/json"
	"fmt"

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
		if key == "success" || key == "status" || key == authboss.DataErr || key == authboss.DataValidation {
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

	envelope := authboss.HTMLData{
		"success": success,
		"status":  status,
	}
	if success {
		envelope["data"] = payload
	} else {
		envelope["error"] = authErrorMessage(data)
		if fields, ok := data[authboss.DataValidation]; ok && fields != nil {
			envelope["fields"] = fields
		}
	}

	body, err := json.Marshal(envelope)
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

func authErrorMessage(data authboss.HTMLData) string {
	if errValue, ok := data[authboss.DataErr]; ok && errValue != nil {
		return fmt.Sprint(errValue)
	}
	if _, ok := data[authboss.DataValidation]; ok {
		return "validation failed"
	}

	return "request failed"
}
