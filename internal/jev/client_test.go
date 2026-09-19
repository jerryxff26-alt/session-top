package jev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAssessRequestAndResponse(t *testing.T) {
	const apiKey = "test-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/v1/systemone" {
			t.Errorf("path = %q, want /v1/systemone", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+apiKey {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}

		var request struct {
			State     map[string]any      `json:"state"`
			Model     string              `json:"model"`
			Questions map[string]question `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "jev-latest" {
			t.Errorf("model = %q", request.Model)
		}
		if request.State["session_id"] != "session-1" {
			t.Errorf("state = %#v", request.State)
		}
		wantTypes := map[string]string{
			"distill_priority":   "choice",
			"primary_value":      "choice",
			"reusable_knowledge": "noul",
			"verified_evidence":  "noul",
			"correction_value":   "noul",
		}
		if len(request.Questions) != len(wantTypes) {
			t.Errorf("question count = %d, want %d", len(request.Questions), len(wantTypes))
		}
		for name, wantType := range wantTypes {
			got, ok := request.Questions[name]
			if !ok {
				t.Errorf("missing question %q", name)
				continue
			}
			if got.Type != wantType {
				t.Errorf("question %q type = %q, want %q", name, got.Type, wantType)
			}
			if got.Instructions == "" {
				t.Errorf("question %q has empty instructions", name)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"model":"jev-latest",
			"answers":{
				"distill_priority":{"type":"choice","choice":"high","confidence":0.91,"probabilities":{"high":0.91,"medium":0.06,"low":0.02,"none":0.01}},
				"primary_value":{"type":"choice","choice":"correction_value","confidence":0.8,"probabilities":{"reusable_knowledge":0.1,"verified_evidence":0.09,"correction_value":0.8,"none":0.01}},
				"reusable_knowledge":{"type":"noul","noul":0.72},
				"verified_evidence":{"type":"noul","noul":0.63},
				"correction_value":{"type":"noul","noul":0.96}
			},
			"usage":{"input_tokens":321,"output_tokens":45}
		}`)
	}))
	defer server.Close()

	client, err := New(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = server.URL + "/v1/systemone"

	got, err := client.Assess(context.Background(), map[string]any{
		"session_id": "session-1",
		"messages":   []string{"first", "second"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "jev-latest" {
		t.Errorf("model = %q", got.Model)
	}
	if got.DistillPriority.Choice != "high" || got.DistillPriority.Confidence != 0.91 {
		t.Errorf("distill priority = %#v", got.DistillPriority)
	}
	if got.PrimaryValue.Choice != "correction_value" {
		t.Errorf("primary value = %#v", got.PrimaryValue)
	}
	if got.ReusableKnowledge.Noul != 0.72 || got.VerifiedEvidence.Noul != 0.63 || got.CorrectionValue.Noul != 0.96 {
		t.Errorf("noul answers = %#v %#v %#v", got.ReusableKnowledge, got.VerifiedEvidence, got.CorrectionValue)
	}
	if got.Usage.InputTokens != 321 || got.Usage.OutputTokens != 45 {
		t.Errorf("usage = %#v", got.Usage)
	}

	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		`"distill_priority"`, `"primary_value"`, `"reusable_knowledge"`,
		`"verified_evidence"`, `"correction_value"`, `"usage"`,
	} {
		if !strings.Contains(string(encoded), field) {
			t.Errorf("assessment JSON missing %s: %s", field, encoded)
		}
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Run("JEV_API_KEY takes priority", func(t *testing.T) {
		t.Setenv("JEV_API_KEY", "primary")
		t.Setenv("TYPESAFE_API_KEY", "fallback")
		client, err := NewFromEnv()
		if err != nil {
			t.Fatal(err)
		}
		if client.apiKey != "primary" {
			t.Fatalf("selected wrong environment key")
		}
		if client.httpClient.Timeout != 30*time.Second {
			t.Fatalf("timeout = %s, want 30s", client.httpClient.Timeout)
		}
	})

	t.Run("compatibility fallback", func(t *testing.T) {
		t.Setenv("JEV_API_KEY", "")
		t.Setenv("TYPESAFE_API_KEY", "fallback")
		client, err := NewFromEnv()
		if err != nil {
			t.Fatal(err)
		}
		if client.apiKey != "fallback" {
			t.Fatalf("compatibility key was not selected")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		t.Setenv("JEV_API_KEY", "")
		t.Setenv("TYPESAFE_API_KEY", "")
		_, err := NewFromEnv()
		if !errors.Is(err, ErrMissingAPIKey) {
			t.Fatalf("error = %v, want ErrMissingAPIKey", err)
		}
	})
}

func TestAssessNon2xx(t *testing.T) {
	const apiKey = "test-secret"
	largeBody := "upstream rejected request for test-secret: " + strings.Repeat("x", maxErrorBody+100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, largeBody)
	}))
	defer server.Close()

	client, err := New(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = server.URL

	_, err = client.Assess(context.Background(), map[string]string{"id": "session-1"})
	if err == nil {
		t.Fatal("expected non-2xx error")
	}
	message := err.Error()
	if !strings.Contains(message, "422 Unprocessable Entity") {
		t.Errorf("error lacks status: %s", message)
	}
	if !strings.Contains(message, "upstream rejected request") {
		t.Errorf("error lacks response detail: %s", message)
	}
	if strings.Contains(message, apiKey) {
		t.Errorf("error leaked API key: %s", message)
	}
	if len(message) > maxErrorBody+100 {
		t.Errorf("error body was not bounded: len=%d", len(message))
	}
}
