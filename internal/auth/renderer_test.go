package auth

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aarondl/authboss/v3"
)

func TestSuccessJSONRendererAddsSuccessTrue(t *testing.T) {
	renderer := newSuccessJSONRenderer()

	body, contentType, err := renderer.Render(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("content type = %q, want %q", contentType, "application/json")
	}

	data := map[string]any{}
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if data["success"] != true {
		t.Fatalf("success = %v, want true", data["success"])
	}
	if data["status"] != "success" {
		t.Fatalf("status = %v, want success", data["status"])
	}
	payload, ok := data["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %T, want object", data["data"])
	}
	if len(payload) != 0 {
		t.Fatalf("data = %v, want empty object", payload)
	}
}

func TestSuccessJSONRendererAddsSuccessFalseForFailures(t *testing.T) {
	renderer := newSuccessJSONRenderer()

	body, _, err := renderer.Render(context.Background(), "test", authboss.HTMLData{
		authboss.DataErr: "problem",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	data := map[string]any{}
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if data["success"] != false {
		t.Fatalf("success = %v, want false", data["success"])
	}
	if data["status"] != "failure" {
		t.Fatalf("status = %v, want failure", data["status"])
	}
	if data["error"] != "problem" {
		t.Fatalf("error = %v, want problem", data["error"])
	}
	if _, ok := data["data"]; ok {
		t.Fatalf("data = %v, want absent", data["data"])
	}
}
