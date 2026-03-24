package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRespondOK(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	respondOK(w, data)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp V2APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
	if resp.Error != nil {
		t.Errorf("expected no error, got %+v", resp.Error)
	}
	if resp.Data == nil {
		t.Error("expected data to be present")
	}
}

func TestRespondOKNilData(t *testing.T) {
	w := httptest.NewRecorder()
	respondOK(w, nil)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp V2APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()
	respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "something went wrong")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp V2APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", resp.Status)
	}
	if resp.Error == nil {
		t.Fatal("expected error to be present")
	}
	if resp.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("expected error code 'INTERNAL_ERROR', got '%s'", resp.Error.Code)
	}
	if resp.Error.Message != "something went wrong" {
		t.Errorf("expected error message 'something went wrong', got '%s'", resp.Error.Message)
	}
	if resp.Data != nil {
		t.Errorf("expected no data in error response, got %+v", resp.Data)
	}
}

func TestRespondNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	respondNotFound(w, "resource not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var resp V2APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", resp.Status)
	}
	if resp.Error == nil {
		t.Fatal("expected error to be present")
	}
	if resp.Error.Code != "NOT_FOUND" {
		t.Errorf("expected error code 'NOT_FOUND', got '%s'", resp.Error.Code)
	}
	if resp.Error.Message != "resource not found" {
		t.Errorf("expected error message 'resource not found', got '%s'", resp.Error.Message)
	}
}

func TestRespondErrorBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	respondError(w, http.StatusBadRequest, "INVALID_INPUT", "bad input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp V2APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Error.Code != "INVALID_INPUT" {
		t.Errorf("expected error code 'INVALID_INPUT', got '%s'", resp.Error.Code)
	}
}

func TestV2APIResponseJSONOmitempty(t *testing.T) {
	resp := V2APIResponse{Status: "ok"}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := raw["data"]; ok {
		t.Error("expected 'data' to be omitted when nil")
	}
	if _, ok := raw["error"]; ok {
		t.Error("expected 'error' to be omitted when nil")
	}
	if _, ok := raw["message"]; ok {
		t.Error("expected 'message' to be omitted when empty")
	}
	if _, ok := raw["status"]; !ok {
		t.Error("expected 'status' to be present")
	}
}

func TestRespondOKContentType(t *testing.T) {
	w := httptest.NewRecorder()
	respondOK(w, "test")

	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=UTF-8" {
		t.Errorf("expected Content-Type 'application/json; charset=UTF-8', got '%s'", ct)
	}
}
