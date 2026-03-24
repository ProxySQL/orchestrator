/*
   Copyright 2026 Percona LLC

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

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
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var resp V2APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
	if resp.Error != nil {
		t.Errorf("expected nil error, got %+v", resp.Error)
	}
	if resp.Data == nil {
		t.Error("expected data to be non-nil")
	}
	// Verify the data content
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", resp.Data)
	}
	if dataMap["key"] != "value" {
		t.Errorf("expected data[key]='value', got '%v'", dataMap["key"])
	}
}

func TestRespondOKWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	respondOKWithMessage(w, []int{1, 2, 3}, "found items")

	var resp V2APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
	if resp.Message != "found items" {
		t.Errorf("expected message 'found items', got '%s'", resp.Message)
	}
	if resp.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()
	respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "something went wrong")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var resp V2APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", resp.Status)
	}
	if resp.Data != nil {
		t.Errorf("expected nil data on error, got %+v", resp.Data)
	}
	if resp.Error == nil {
		t.Fatal("expected error field to be non-nil")
	}
	if resp.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("expected error code 'INTERNAL_ERROR', got '%s'", resp.Error.Code)
	}
	if resp.Error.Message != "something went wrong" {
		t.Errorf("expected error message 'something went wrong', got '%s'", resp.Error.Message)
	}
}

func TestRespondNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	respondNotFound(w, "resource not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var resp V2APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", resp.Status)
	}
	if resp.Error == nil {
		t.Fatal("expected error field to be non-nil")
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
	respondError(w, http.StatusBadRequest, "INVALID_PARAMS", "bad input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp V2APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Status != "error" {
		t.Errorf("expected status 'error', got '%s'", resp.Status)
	}
	if resp.Error.Code != "INVALID_PARAMS" {
		t.Errorf("expected error code 'INVALID_PARAMS', got '%s'", resp.Error.Code)
	}
}

func TestResponseEnvelopeOmitsNilFields(t *testing.T) {
	w := httptest.NewRecorder()
	respondOK(w, "hello")

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	// "error" should be omitted (omitempty)
	if _, exists := raw["error"]; exists {
		t.Error("expected 'error' field to be omitted when nil")
	}
	// "message" should be omitted when empty
	if _, exists := raw["message"]; exists {
		t.Error("expected 'message' field to be omitted when empty")
	}
	// "data" should be present
	if _, exists := raw["data"]; !exists {
		t.Error("expected 'data' field to be present")
	}
	// "status" should be present
	if _, exists := raw["status"]; !exists {
		t.Error("expected 'status' field to be present")
	}
}

func TestResponseEnvelopeErrorOmitsData(t *testing.T) {
	w := httptest.NewRecorder()
	respondError(w, http.StatusInternalServerError, "ERR", "fail")

	var raw map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	// "data" should be omitted on error
	if _, exists := raw["data"]; exists {
		t.Error("expected 'data' field to be omitted on error")
	}
	// "error" should be present
	if _, exists := raw["error"]; !exists {
		t.Error("expected 'error' field to be present")
	}
}

func TestRegisterV2Routes(t *testing.T) {
	router := RegisterV2Routes()
	if router == nil {
		t.Fatal("expected non-nil router from RegisterV2Routes")
	}
}
