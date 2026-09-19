// Package jev provides a small HTTP client for assessing the distillation value
// of structured Codex session state with TypeSafe Jev.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultEndpoint = "https://api.typesafe.ai/v1/systemone"
	defaultModel    = "jev-latest"
	maxErrorBody    = 4 << 10
)

// ErrMissingAPIKey is returned when neither supported API key source is set.
var ErrMissingAPIKey = errors.New("jev: API key is required (set JEV_API_KEY or TYPESAFE_API_KEY)")

// Client calls the TypeSafe System One API.
type Client struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

// New constructs a client with the supplied API key.
func New(apiKey string) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}
	return &Client{
		apiKey:   apiKey,
		endpoint: defaultEndpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// NewFromEnv constructs a client from JEV_API_KEY, falling back to
// TYPESAFE_API_KEY for compatibility.
func NewFromEnv() (*Client, error) {
	apiKey := strings.TrimSpace(os.Getenv("JEV_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY"))
	}
	return New(apiKey)
}

// Assessment is the stable, caller-facing distillation-value result.
type Assessment struct {
	Model             string       `json:"model"`
	DistillPriority   ChoiceAnswer `json:"distill_priority"`
	PrimaryValue      ChoiceAnswer `json:"primary_value"`
	ReusableKnowledge NoulAnswer   `json:"reusable_knowledge"`
	VerifiedEvidence  NoulAnswer   `json:"verified_evidence"`
	CorrectionValue   NoulAnswer   `json:"correction_value"`
	Usage             Usage        `json:"usage"`
}

// ChoiceAnswer is a TypeSafe choice result.
type ChoiceAnswer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// NoulAnswer is a TypeSafe yes/no probability result.
type NoulAnswer struct {
	Type string  `json:"type"`
	Noul float64 `json:"noul"`
}

// Usage reports tokens consumed by the assessment request.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"`
}

type assessRequest struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]question `json:"questions"`
}

type assessResponse struct {
	Model   string `json:"model"`
	Answers struct {
		DistillPriority   ChoiceAnswer `json:"distill_priority"`
		PrimaryValue      ChoiceAnswer `json:"primary_value"`
		ReusableKnowledge NoulAnswer   `json:"reusable_knowledge"`
		VerifiedEvidence  NoulAnswer   `json:"verified_evidence"`
		CorrectionValue   NoulAnswer   `json:"correction_value"`
	} `json:"answers"`
	Usage Usage `json:"usage"`
}

// Assess evaluates structured state using a fixed set of distillation-value
// questions. State may be any JSON-marshalable value.
func (c *Client) Assess(ctx context.Context, state any) (Assessment, error) {
	if c == nil || strings.TrimSpace(c.apiKey) == "" {
		return Assessment{}, ErrMissingAPIKey
	}

	payload, err := json.Marshal(assessRequest{
		State:     state,
		Model:     defaultModel,
		Questions: distillationQuestions(),
	})
	if err != nil {
		return Assessment{}, fmt.Errorf("jev: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return Assessment{}, fmt.Errorf("jev: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Assessment{}, fmt.Errorf("jev: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		if readErr != nil {
			return Assessment{}, fmt.Errorf("jev: unexpected HTTP status %s (read response: %v)", resp.Status, readErr)
		}
		detail := strings.TrimSpace(string(body))
		if c.apiKey != "" {
			detail = strings.ReplaceAll(detail, c.apiKey, "[REDACTED]")
		}
		if detail == "" {
			return Assessment{}, fmt.Errorf("jev: unexpected HTTP status %s", resp.Status)
		}
		return Assessment{}, fmt.Errorf("jev: unexpected HTTP status %s: %s", resp.Status, detail)
	}

	var decoded assessResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return Assessment{}, fmt.Errorf("jev: decode response: %w", err)
	}

	return Assessment{
		Model:             decoded.Model,
		DistillPriority:   decoded.Answers.DistillPriority,
		PrimaryValue:      decoded.Answers.PrimaryValue,
		ReusableKnowledge: decoded.Answers.ReusableKnowledge,
		VerifiedEvidence:  decoded.Answers.VerifiedEvidence,
		CorrectionValue:   decoded.Answers.CorrectionValue,
		Usage:             decoded.Usage,
	}, nil
}

func distillationQuestions() map[string]question {
	return map[string]question{
		"distill_priority": {
			Type:         "choice",
			Instructions: "What priority should this session have for distillation into reusable project knowledge?",
			Criteria: map[string]string{
				"high":   "Contains important reusable knowledge, verified decisions, or corrections that are likely to save substantial future work.",
				"medium": "Contains useful project-specific knowledge, but its reuse value or evidence is limited.",
				"low":    "The complete context shows little reusable knowledge or mostly routine work.",
				"none":   "The complete context shows no meaningful knowledge worth distilling.",
				"review": "Context coverage is incomplete, truncated, or ambiguous, so a human or stronger model must review it before assigning low or no value.",
			},
		},
		"primary_value": {
			Type:         "choice",
			Instructions: "What is the primary source of distillation value in this session?",
			Criteria: map[string]string{
				"reusable_knowledge": "Reusable repository facts, procedures, constraints, or implementation knowledge.",
				"verified_evidence":  "Commands, tests, observations, or other evidence that verifies a conclusion.",
				"correction_value":   "A correction, failed approach, or explicit do-not-repeat lesson.",
				"none":               "No meaningful distillation value is present.",
			},
		},
		"reusable_knowledge": {
			Type:         "noul",
			Instructions: "Does this session contain specific knowledge that is reusable in future work on the same project?",
		},
		"verified_evidence": {
			Type:         "noul",
			Instructions: "Does this session contain concrete evidence that verifies its important conclusions?",
		},
		"correction_value": {
			Type:         "noul",
			Instructions: "Does this session contain a correction, failed approach, or lesson that would prevent repeated mistakes?",
		},
	}
}
