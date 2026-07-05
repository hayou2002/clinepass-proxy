package internal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// ============================================================
// ClinePass Proxy v2.0.0 - Core Proxy Logic
// ============================================================

const (
	clinepassBaseURL = "https://api.cline.bot/api/v1"
	defaultTimeout   = 300 * time.Second
)

// Proxy handles request forwarding and reasoning field conversion
type Proxy struct {
	APIKey string
	Debug  bool
	Client *http.Client
}

// NewProxy creates a new proxy instance
func NewProxy(apiKey string, debug bool) *Proxy {
	return &Proxy{
		Client: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

func (p *Proxy) debugf(format string, args ...interface{}) {
	if p.Debug {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// reasoningFields lists all known reasoning field names from different providers
var reasoningFields = []string{"reasoning", "reasoning_details", "thinking"}

// moveReasoningToStandard converts any reasoning field to reasoning_content
func moveReasoningToStandard(m map[string]interface{}) {
	for _, field := range reasoningFields {
		if val, exists := m[field]; exists {
			if str, ok := val.(string); ok && str != "" {
				if _, has := m["reasoning_content"]; !has {
					m["reasoning_content"] = str
				}
				delete(m, field)
			}
		}
	}
}

// convertReasoningInChoices processes choices array (works for both delta and message)
func convertReasoningInChoices(choices []interface{}) {
	for _, choice := range choices {
		if choiceMap, ok := choice.(map[string]interface{}); ok {
			// Handle streaming delta
			if delta, ok := choiceMap["delta"].(map[string]interface{}); ok {
				moveReasoningToStandard(delta)
			}
			// Handle non-streaming message
			if msg, ok := choiceMap["message"].(map[string]interface{}); ok {
				moveReasoningToStandard(msg)
			}
		}
	}
}

// injectReasoningParams adds provider-specific reasoning parameters
func injectReasoningParams(reqMap map[string]interface{}, model string) {
	modelLower := strings.ToLower(model)

	if strings.Contains(modelLower, "minimax") {
		// MiniMax M3 needs reasoning_split to output thinking content
		if _, ok := reqMap["reasoning_split"]; !ok {
			reqMap["reasoning_split"] = true
		}
	}

	if strings.Contains(modelLower, "deepseek") {
		// DeepSeek V4 supports thinking mode toggle
		if _, ok := reqMap["thinking"]; !ok {
			reqMap["thinking"] = map[string]string{"type": "enabled"}
		}
	}

	// Default to medium reasoning effort.
	// Client can override with reasoning_effort: "high" in request body.
	if _, ok := reqMap["reasoning_effort"]; !ok {
		reqMap["reasoning_effort"] = "medium"
	}
}



// HandleChatCompletions handles /v1/chat/completions
func (p *Proxy) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// API key: server config > client header
	apiKey := p.APIKey
	if apiKey == "" {
		if clientKey := r.Header.Get("Authorization"); clientKey != "" {
			apiKey = strings.TrimPrefix(clientKey, "Bearer ")
			apiKey = strings.TrimSpace(apiKey)
		}
	}
	if apiKey == "" {
		p.writeError(w, http.StatusUnauthorized, "API key required")
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	p.debugf("Request body: %s", string(body))

	// Parse request as raw map for processing
	var reqMap map[string]interface{}
	if err := json.Unmarshal(body, &reqMap); err != nil {
		p.writeError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	// Get model name for provider-specific injection
	model, _ := reqMap["model"].(string)

	// Inject reasoning parameters based on model
	injectReasoningParams(reqMap, model)



	// Re-marshal modified request
	finalBody, err := json.Marshal(reqMap)
	if err != nil {
		p.writeError(w, http.StatusInternalServerError, "Failed to process request")
		return
	}

	p.debugf("Forwarding to ClinePass: %s", string(finalBody))

	// Forward to ClinePass
	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		clinepassBaseURL+"/chat/completions", bytes.NewReader(finalBody))
	if err != nil {
		p.writeError(w, http.StatusInternalServerError, "Failed to create upstream request")
		return
	}

	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	upstreamReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.Client.Do(upstreamReq)
	if err != nil {
		p.writeError(w, http.StatusBadGateway, "Upstream connection error: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		log.Printf("[ERROR] ClinePass returned %d: %s", resp.StatusCode, string(errBody))
		p.writeError(w, resp.StatusCode, "ClinePass error: "+string(errBody))
		return
	}

	// Determine if streaming
	isStream := false
	if s, ok := reqMap["stream"].(bool); ok {
		isStream = s
	}

	if isStream {
		p.handleStream(w, r, resp)
	} else {
		p.handleNonStream(w, resp)
	}
}

// handleStream processes streaming SSE response with reasoning field conversion
func (p *Proxy) handleStream(w http.ResponseWriter, r *http.Request, upstream *http.Response) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		p.writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	scanner := bufio.NewScanner(upstream.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			continue
		}

		// Parse, convert reasoning fields, re-marshal
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Can't parse - pass through as-is
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			continue
		}

		// Convert reasoning fields in delta
		if choices, ok := chunk["choices"].([]interface{}); ok {
			convertReasoningInChoices(choices)
		}

		out, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", string(out))
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		log.Printf("[ERROR] Stream scan: %v", err)
	}
}

// handleNonStream processes non-streaming JSON response
func (p *Proxy) handleNonStream(w http.ResponseWriter, upstream *http.Response) {
	body, err := io.ReadAll(upstream.Body)
	if err != nil {
		p.writeError(w, http.StatusBadGateway, "Failed to read upstream response")
		return
	}

	p.debugf("ClinePass response: %s", string(body))

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		// Can't parse - pass through raw
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
		return
	}

	// Convert reasoning fields in message
	if choices, ok := resp["choices"].([]interface{}); ok {
		convertReasoningInChoices(choices)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleModels returns the ClinePass model catalog with full metadata
func (p *Proxy) HandleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ModelList{
		Object: "list",
		Data:   ClinePassModels,
	})
}

// HandleHealth returns service health status
func (p *Proxy) HandleHealth(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","version":"%s"}`, version)
	}
}

func (p *Proxy) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "invalid_request_error",
		},
	})
}
