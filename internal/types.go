package internal

// ============================================================
// ClinePass Proxy v2.0.0 - Type Definitions
// ============================================================

// ErrorResponse follows OpenAI error format
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string      `json:"message"`
	Type    string      `json:"type"`
	Param   interface{} `json:"param"`
	Code    interface{} `json:"code"`
}

// ============================================================
// Model Catalog - ClinePass official models with full metadata
// ============================================================

type ModelList struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

type Model struct {
	ID              string   `json:"id"`
	Object          string   `json:"object"`
	Created         int64    `json:"created"`
	OwnedBy         string   `json:"owned_by"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	ContextLength   int      `json:"context_length"`
	SupportsReason  bool     `json:"supports_reasoning"`
	SupportsVision  bool     `json:"supports_vision"`
	SupportsTools   bool     `json:"supports_tools"`
	SupportsAudio   bool     `json:"supports_audio"`
	SupportsVideo   bool     `json:"supports_video"`
	InputPrice      float64  `json:"input_price_per_million"`
	OutputPrice     float64  `json:"output_price_per_million"`
	CachedReadPrice float64  `json:"cached_read_price_per_million"`
	Tags            []string `json:"tags,omitempty"`
}

// ClinePass official model catalog
// Source: https://docs.cline.bot/getting-started/clinepass
// Individual model specs sourced from each provider's official documentation.
var ClinePassModels = []Model{
	{
		ID: "cline-pass/glm-5.2", Object: "model", Created: 0, OwnedBy: "zhipuai",
		Name: "GLM-5.2",
		Description: "ZhipuAI GLM-5.2 - 1M context, vision & reasoning, agentic coding",
		ContextLength: 1000000, SupportsReason: true, SupportsVision: true,
		SupportsTools: true, SupportsAudio: false, SupportsVideo: false,
		InputPrice: 1.40, OutputPrice: 4.40, CachedReadPrice: 0.26,
		Tags: []string{"coding", "reasoning", "vision", "agentic", "1M-context"},
	},
	{
		ID: "cline-pass/kimi-k2.7-code", Object: "model", Created: 0, OwnedBy: "moonshotai",
		Name: "Kimi K2.7 Code",
		Description: "MoonshotAI Kimi K2.7 Code - ~1T MoE, coding-specialist, 256K ctx, image input",
		ContextLength: 262144, SupportsReason: true, SupportsVision: true,
		SupportsTools: true, SupportsAudio: false, SupportsVideo: false,
		InputPrice: 0.95, OutputPrice: 4.00, CachedReadPrice: 0.19,
		Tags: []string{"coding", "reasoning", "vision", "agentic", "256K-context"},
	},
	{
		ID: "cline-pass/kimi-k2.6", Object: "model", Created: 0, OwnedBy: "moonshotai",
		Name: "Kimi K2.6",
		Description: "MoonshotAI Kimi K2.6 - latest smart model, deep thinking, 262K ctx, image input",
		ContextLength: 262144, SupportsReason: true, SupportsVision: true,
		SupportsTools: true, SupportsAudio: false, SupportsVideo: false,
		InputPrice: 0.95, OutputPrice: 4.00, CachedReadPrice: 0.16,
		Tags: []string{"reasoning", "coding", "vision", "deep-thinking", "262K-context"},
	},
	{
		ID: "cline-pass/deepseek-v4-pro", Object: "model", Created: 0, OwnedBy: "deepseek",
		Name: "DeepSeek V4 Pro",
		Description: "DeepSeek V4 Pro - 1.6T MoE, frontier thinking mode, 1M ctx, coding & agentic",
		ContextLength: 1000000, SupportsReason: true, SupportsVision: false,
		SupportsTools: true, SupportsAudio: false, SupportsVideo: false,
		InputPrice: 1.74, OutputPrice: 3.48, CachedReadPrice: 0.0145,
		Tags: []string{"reasoning", "thinking-mode", "coding", "1M-context", "agentic"},
	},
	{
		ID: "cline-pass/deepseek-v4-flash", Object: "model", Created: 0, OwnedBy: "deepseek",
		Name: "DeepSeek V4 Flash",
		Description: "DeepSeek V4 Flash - 284B MoE, fast & cheap, thinking mode, 1M ctx",
		ContextLength: 1000000, SupportsReason: true, SupportsVision: false,
		SupportsTools: true, SupportsAudio: false, SupportsVideo: false,
		InputPrice: 0.14, OutputPrice: 0.28, CachedReadPrice: 0.0028,
		Tags: []string{"reasoning", "thinking-mode", "fast", "cheap", "1M-context", "high-tps"},
	},
	{
		ID: "cline-pass/mimo-v2.5", Object: "model", Created: 0, OwnedBy: "xiaomi",
		Name: "MiMo-V2.5",
		Description: "Xiaomi MiMo V2.5 - lightweight MoE, 1M ctx, all modalities: vision+video+audio",
		ContextLength: 1048576, SupportsReason: true, SupportsVision: true,
		SupportsTools: true, SupportsAudio: true, SupportsVideo: true,
		InputPrice: 0.14, OutputPrice: 0.28, CachedReadPrice: 0.0028,
		Tags: []string{"reasoning", "vision", "video", "audio", "multimodal", "fast", "cheap", "1M-context"},
	},
	{
		ID: "cline-pass/mimo-v2.5-pro", Object: "model", Created: 0, OwnedBy: "xiaomi",
		Name: "MiMo-V2.5-Pro",
		Description: "Xiaomi MiMo V2.5 Pro - 1.02T MoE flagship, 1M ctx, all modalities: vision+video+audio",
		ContextLength: 1048576, SupportsReason: true, SupportsVision: true,
		SupportsTools: true, SupportsAudio: true, SupportsVideo: true,
		InputPrice: 1.74, OutputPrice: 3.48, CachedReadPrice: 0.0145,
		Tags: []string{"reasoning", "vision", "video", "audio", "multimodal", "coding", "1M-context"},
	},
	{
		ID: "cline-pass/minimax-m3", Object: "model", Created: 0, OwnedBy: "minimaxai",
		Name: "MiniMax M3",
		Description: "MiniMax M3 - multimodal MoE, 1M ctx, vision+video+reasoning+agentic coding",
		ContextLength: 1000000, SupportsReason: true, SupportsVision: true,
		SupportsTools: true, SupportsAudio: false, SupportsVideo: true,
		InputPrice: 0.30, OutputPrice: 1.20, CachedReadPrice: 0.06,
		Tags: []string{"multimodal", "vision", "video", "reasoning", "1M-context", "agentic", "coding"},
	},
	{
		ID: "cline-pass/qwen3.7-max", Object: "model", Created: 0, OwnedBy: "qwen",
		Name: "Qwen3.7 Max",
		Description: "Alibaba Qwen3.7 Max - flagship agent model, 1M ctx, frontier reasoning & coding",
		ContextLength: 1000000, SupportsReason: true, SupportsVision: false,
		SupportsTools: true, SupportsAudio: false, SupportsVideo: false,
		InputPrice: 2.50, OutputPrice: 7.50, CachedReadPrice: 0.50,
		Tags: []string{"reasoning", "agentic", "coding", "1M-context", "long-horizon"},
	},
	{
		ID: "cline-pass/qwen3.7-plus", Object: "model", Created: 0, OwnedBy: "qwen",
		Name: "Qwen3.7 Plus",
		Description: "Alibaba Qwen3.7 Plus - cost-effective, 1M ctx, balanced performance",
		ContextLength: 1000000, SupportsReason: true, SupportsVision: false,
		SupportsTools: true, SupportsAudio: false, SupportsVideo: false,
		InputPrice: 0.40, OutputPrice: 1.60, CachedReadPrice: 0.04,
		Tags: []string{"reasoning", "coding", "1M-context", "cost-effective"},
	},
}
