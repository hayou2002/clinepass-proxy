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

const (
	clinepassBaseURL = "https://api.cline.bot/api/v1"
	defaultTimeout   = 300 * time.Second
)

var reasoningFields = []string{"reasoning", "reasoning_details", "thinking"}

type Proxy struct {
	APIKey       string
	Debug        bool
	ThinkingLang string
	Client       *http.Client
}

func NewProxy(apiKey string, debug bool, thinkingLang string) *Proxy {
	if thinkingLang == "" {
		thinkingLang = "zh"
	}
	return &Proxy{
		APIKey:       apiKey,
		Debug:        debug,
		ThinkingLang: thinkingLang,
		Client:       &http.Client{Timeout: defaultTimeout},
	}
}

func (p *Proxy) debugf(f string, a ...interface{}) {
	if p.Debug {
		log.Printf("[DEBUG] "+f, a...)
	}
}

func moveReasoningToStandard(m map[string]interface{}) {
	for _, f := range reasoningFields {
		if v, ok := m[f]; ok {
			if s, ok2 := v.(string); ok2 && s != "" {
				if _, hasRC := m["reasoning_content"]; !hasRC {
					m["reasoning_content"] = s
				}
				delete(m, f)
			}
		}
	}
}

func convertReasoningInChoices(choices []interface{}) {
	for _, c := range choices {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if d, ok2 := cm["delta"].(map[string]interface{}); ok2 {
			moveReasoningToStandard(d)
		}
		if msg, ok2 := cm["message"].(map[string]interface{}); ok2 {
			moveReasoningToStandard(msg)
		}
	}
}

func injectReasoningParams(m map[string]interface{}, model string) {
	ml := strings.ToLower(model)
	if strings.Contains(ml, "minimax") {
		if _, ok := m["reasoning_split"]; !ok {
			m["reasoning_split"] = true
		}
	}
	if strings.Contains(ml, "deepseek") {
		if _, ok := m["thinking"]; !ok {
			m["thinking"] = map[string]string{"type": "enabled"}
		}
	}
	if _, ok := m["reasoning_effort"]; !ok {
		m["reasoning_effort"] = "high"
	}
}

func injectThinkingLanguage(m map[string]interface{}, lang string) {
	if lang == "" {
		return
	}
	var instruction string
	switch lang {
	case "zh":
		instruction = "请始终使用中文进行思考和推理，无论用户输入的语言、读取的文件内容、代码或网页内容是什么语言，你的内部思考过程（reasoning）必须使用中文。"
	case "en":
		instruction = "Always think and reason in English, regardless of user input, files, code, or web content language."
	default:
		instruction = fmt.Sprintf("Always think and reason in %s.", lang)
	}
	msgs, ok := m["messages"].([]interface{})
	if !ok || len(msgs) == 0 {
		return
	}
	if fm, ok2 := msgs[0].(map[string]interface{}); ok2 {
		if r, _ := fm["role"].(string); r == "system" {
			if c, _ := fm["content"].(string); c != "" {
				fm["content"] = instruction + "\n\n" + c
				return
			}
		}
	}
	m["messages"] = append([]interface{}{
		map[string]interface{}{"role": "system", "content": instruction},
	}, msgs...)
}

func (p *Proxy) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	apiKey := p.APIKey
	if apiKey == "" {
		if ck := r.Header.Get("Authorization"); ck != "" {
			apiKey = strings.TrimPrefix(ck, "Bearer ")
			apiKey = strings.TrimSpace(apiKey)
		}
	}
	if apiKey == "" {
		p.writeError(w, http.StatusUnauthorized, "API key required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeError(w, http.StatusBadRequest, "Failed to read body")
		return
	}
	r.Body.Close()
	p.debugf("Request: %s", string(body))

	var reqMap map[string]interface{}
	if err := json.Unmarshal(body, &reqMap); err != nil {
		p.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	model, _ := reqMap["model"].(string)
	injectReasoningParams(reqMap, model)
	if p.ThinkingLang != "" {
		injectThinkingLanguage(reqMap, p.ThinkingLang)
	}

	finalBody, _ := json.Marshal(reqMap)
	p.debugf("Forward: %s", string(finalBody))

	upReq, _ := http.NewRequestWithContext(r.Context(), "POST",
		clinepassBaseURL+"/chat/completions", bytes.NewReader(finalBody))
	upReq.Header.Set("Content-Type", "application/json")
	upReq.Header.Set("Authorization", "Bearer "+apiKey)
	upReq.Header.Set("Accept", "text/event-stream")

	upResp, err := p.Client.Do(upReq)
	if err != nil {
		p.writeError(w, http.StatusBadGateway, "Upstream error: "+err.Error())
		return
	}
	defer upResp.Body.Close()

	if upResp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(upResp.Body)
		log.Printf("[ERROR] ClinePass %d: %s", upResp.StatusCode, string(errBody))
		p.writeError(w, upResp.StatusCode, "ClinePass error: "+string(errBody))
		return
	}

	isStream, _ := reqMap["stream"].(bool)
	if isStream {
		p.handleStream(w, r, upResp)
	} else {
		p.handleNonStream(w, upResp)
	}
}

func (p *Proxy) handleStream(w http.ResponseWriter, r *http.Request, up *http.Response) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		p.writeError(w, http.StatusInternalServerError, "Streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	sc := bufio.NewScanner(up.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		select {
		case <-r.Context().Done():
			return
		default:
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			continue
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			continue
		}
		if cs, ok := chunk["choices"].([]interface{}); ok {
			convertReasoningInChoices(cs)
		}
		out, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", string(out))
		flusher.Flush()
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		log.Printf("[ERROR] Stream: %v", err)
	}
}

func (p *Proxy) handleNonStream(w http.ResponseWriter, up *http.Response) {
	body, err := io.ReadAll(up.Body)
	if err != nil {
		p.writeError(w, http.StatusBadGateway, "Read error")
		return
	}
	p.debugf("Response: %s", string(body))
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
		return
	}
	if cs, ok := resp["choices"].([]interface{}); ok {
		convertReasoningInChoices(cs)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (p *Proxy) HandleModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ModelList{Object: "list", Data: ClinePassModels})
}

func (p *Proxy) HandleHealth(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","version":"%s","thinking_lang":"%s"}`, version, p.ThinkingLang)
	}
}

func (p *Proxy) writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: ErrorDetail{Message: msg, Type: "error"}})
}
