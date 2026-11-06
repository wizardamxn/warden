// Package llm turns a transcript into study material with Gemini.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"warden/internal/core"
)

const (
	defaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	model          = "gemini-3.1-flash-lite"

	// Paid prices per million tokens, from Google's pricing page on 2026-10-31.
	inputUSDPerMillion  = 0.25
	outputUSDPerMillion = 1.50

	// shortcut: fixed exchange rate for cost logs only, read a live rate if money ever depends on it.
	rupeesPerDollar = 85

	maxAttempts = 5

	// A long hint from Google means a long outage, not a reason to sit idle.
	maxRetryWait = 2 * time.Minute
)

// These let the pipeline tell students what happened in a plain sentence.
var (
	ErrDailyQuota = errors.New("gemini's daily quota is used up")
	ErrBusy       = errors.New("gemini stayed busy")
	ErrChecks     = errors.New("failed the checks twice")
)

// Client calls Gemini's HTTP API.
type Client struct {
	Key       string
	BaseURL   string
	HTTP      *http.Client
	RetryWait time.Duration // first wait when Gemini is busy, longer each time
}

// NewClient makes a client for the real API.
func NewClient(key string) *Client {
	return &Client{
		Key:       key,
		BaseURL:   defaultBaseURL,
		HTTP:      &http.Client{Timeout: 10 * time.Minute},
		RetryWait: 15 * time.Second,
	}
}

// Usage counts the tokens a call used. OutputTokens includes thinking, which
// Google bills at the output price.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// CostUSD is what these tokens cost at paid prices.
func (u Usage) CostUSD() float64 {
	return (float64(u.InputTokens)*inputUSDPerMillion + float64(u.OutputTokens)*outputUSDPerMillion) / 1_000_000
}

// CostRupees is CostUSD in rupees, roughly.
func (u Usage) CostRupees() float64 {
	return u.CostUSD() * rupeesPerDollar
}

type part struct {
	Text    string `json:"text"`
	Thought bool   `json:"thought,omitempty"`
}

type turn struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

func userTurn(text string) turn  { return turn{Role: "user", Parts: []part{{Text: text}}} }
func modelTurn(text string) turn { return turn{Role: "model", Parts: []part{{Text: text}}} }

type generationConfig struct {
	ResponseMimeType   string          `json:"responseMimeType"`
	ResponseJSONSchema json.RawMessage `json:"responseJsonSchema"`
}

type generateRequest struct {
	SystemInstruction turn             `json:"systemInstruction"`
	Contents          []turn           `json:"contents"`
	GenerationConfig  generationConfig `json:"generationConfig"`
}

type generateResponse struct {
	Candidates []struct {
		Content      turn   `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		ThoughtsTokenCount   int `json:"thoughtsTokenCount"`
	} `json:"usageMetadata"`
}

// generate makes one call and returns the JSON text Gemini wrote.
// Usage comes back even on an error, because a cut-off answer is still billed.
func (c *Client) generate(ctx context.Context, instructions string, contents []turn, schema string) (string, Usage, error) {
	if c.Key == "" {
		return "", Usage{}, errors.New("GEMINI_API_KEY is not set, add it to server/.env")
	}

	body, err := json.Marshal(generateRequest{
		SystemInstruction: turn{Parts: []part{{Text: instructions}}},
		Contents:          contents,
		GenerationConfig: generationConfig{
			ResponseMimeType:   "application/json",
			ResponseJSONSchema: json.RawMessage(schema),
		},
	})
	if err != nil {
		return "", Usage{}, err
	}

	data, err := c.postWithRetries(ctx, body)
	if err != nil {
		return "", Usage{}, err
	}

	var parsed generateResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", Usage{}, fmt.Errorf("read gemini response: %w", err)
	}
	usage := Usage{
		InputTokens:  parsed.UsageMetadata.PromptTokenCount,
		OutputTokens: parsed.UsageMetadata.CandidatesTokenCount + parsed.UsageMetadata.ThoughtsTokenCount,
	}
	if len(parsed.Candidates) == 0 {
		return "", usage, fmt.Errorf("gemini gave no answer: %s", firstChars(string(data), 500))
	}
	candidate := parsed.Candidates[0]
	if candidate.FinishReason != "STOP" {
		return "", usage, fmt.Errorf("gemini stopped early: %s", candidate.FinishReason)
	}

	var text strings.Builder
	for _, p := range candidate.Content.Parts {
		if !p.Thought {
			text.WriteString(p.Text)
		}
	}
	return text.String(), usage, nil
}

// request is one step's question for Gemini and how to check the answer.
type request[T any] struct {
	step         string // for logs and errors
	instructions string
	schema       string
	prompt       string
	check        func(answer *T) []string
	rescue       func(answer *T) bool // optional, a last chance when the fix-up also fails its checks
}

// ask gets an answer and checks it. If the checks fail, Gemini gets the list
// of problems and one chance to fix them, then the step stops.
func ask[T any](ctx context.Context, client *Client, r request[T]) (T, Usage, error) {
	log := core.Log(ctx)
	contents := []turn{userTurn(r.prompt)}

	var nothing T
	var total Usage
	var problems []string
	for attempt := 0; attempt < 2; attempt++ {
		text, usage, err := client.generate(ctx, r.instructions, contents, r.schema)
		total.InputTokens += usage.InputTokens
		total.OutputTokens += usage.OutputTokens
		if err != nil {
			return nothing, total, err
		}

		var answer T
		parseErr := json.Unmarshal([]byte(text), &answer)
		if parseErr != nil {
			problems = []string{"the answer is not valid JSON: " + parseErr.Error()}
		} else if problems = r.check(&answer); len(problems) == 0 {
			return answer, total, nil
		}
		for _, problem := range problems {
			log.Warn("check failed", "attempt", attempt+1, "problem", firstChars(problem, 300))
		}
		if parseErr == nil && attempt == 1 && r.rescue != nil && r.rescue(&answer) {
			return answer, total, nil
		}
		fix := "Your answer has these problems. Send the whole answer again with them fixed:\n- " + strings.Join(problems, "\n- ")
		contents = append(contents, modelTurn(text), userTurn(fix))
	}
	return nothing, total, fmt.Errorf("%s %w: %s", r.step, ErrChecks, strings.Join(problems, "; "))
}

// postWithRetries waits and tries again when Gemini is too busy or a per-minute
// limit is hit, which pass on their own. A used-up daily quota won't pass, so it
// stops at once. Failed calls aren't billed.
func (c *Client) postWithRetries(ctx context.Context, body []byte) ([]byte, error) {
	log := core.Log(ctx)
	for attempt := 1; ; attempt++ {
		log.Debug("gemini call", "attempt", attempt)
		data, status, err := c.post(ctx, body)
		if status != http.StatusServiceUnavailable && status != http.StatusTooManyRequests {
			return data, err
		}

		reason := "busy"
		if status == http.StatusTooManyRequests {
			reason = "per-minute limit"
			if isDailyQuota(data) {
				log.Warn("gemini daily quota used up, not retrying", "status", status, "reason", "daily quota")
				return nil, fmt.Errorf("%w: %v", ErrDailyQuota, err)
			}
		}
		if attempt == maxAttempts {
			log.Warn("gemini still refusing, giving up", "status", status, "reason", reason, "attempt", attempt)
			return nil, fmt.Errorf("%w: %v", ErrBusy, err)
		}

		wait := time.Duration(attempt) * c.RetryWait
		if hint := retryHint(data); hint > 0 {
			wait = min(hint, maxRetryWait)
		}
		log.Warn("gemini refused, retrying", "status", status, "reason", reason, "attempt", attempt, "wait", wait.String())
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// isDailyQuota reads a 429's body. Google names the limit in the violation's
// quotaId, such as GenerateRequestsPerDayPerProjectPerModel-FreeTier or
// GenerateRequestsPerMinutePerProjectPerModel-FreeTier.
func isDailyQuota(body []byte) bool {
	text := strings.ToLower(string(body))
	return strings.Contains(text, "perday") || strings.Contains(text, "per day")
}

// retryHint is the wait Google asks for, if the error body gives one.
func retryHint(body []byte) time.Duration {
	var parsed struct {
		Error struct {
			Details []struct {
				RetryDelay string `json:"retryDelay"`
			} `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return 0
	}
	for _, detail := range parsed.Error.Details {
		if wait, err := time.ParseDuration(detail.RetryDelay); err == nil {
			return wait
		}
	}
	return 0
}

func (c *Client) post(ctx context.Context, body []byte) ([]byte, int, error) {
	url := c.BaseURL + "/models/" + model + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	// In a header, not the URL, so the key can't show up in error messages.
	req.Header.Set("x-goog-api-key", c.Key)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("call gemini: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read gemini response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// The body goes back too, because it says which limit was hit.
		return data, resp.StatusCode, fmt.Errorf("gemini returned %s: %s", resp.Status, firstChars(string(data), 500))
	}
	return data, resp.StatusCode, nil
}

func firstChars(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
