package proxy

import (
	"regexp"
	"strings"
)

var qwen4BPattern = regexp.MustCompile(`(?i)^qwen.*4b`)

type ModelConfig struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	SupportsVision bool   `json:"supports_vision"`
}

var Models = []ModelConfig{
	{ID: "glm-4.6", Name: "GLM-4.6", Description: "智谱 GLM-4.6", SupportsVision: true},
	{ID: "glm-4.7", Name: "GLM-4.7", Description: "智谱 GLM-4.7", SupportsVision: true},
	{ID: "glm-5", Name: "GLM-5", Description: "智谱 GLM-5 (推荐)", SupportsVision: true},
	{ID: "iFlow-ROME-30BA3B", Name: "iFlow-ROME-30BA3B", Description: "iFlow ROME 30B (快速)", SupportsVision: true},
	{ID: "deepseek-v3.2-chat", Name: "DeepSeek-V3.2", Description: "DeepSeek V3.2 对话模型", SupportsVision: true},
	{ID: "qwen3-coder-plus", Name: "Qwen3-Coder-Plus", Description: "通义千问 Qwen3 Coder Plus", SupportsVision: true},
	{ID: "kimi-k2", Name: "Kimi-K2", Description: "Moonshot Kimi K2", SupportsVision: true},
	{ID: "kimi-k2-thinking", Name: "Kimi-K2-Thinking", Description: "Moonshot Kimi K2 思考模型", SupportsVision: true},
	{ID: "kimi-k2.5", Name: "Kimi-K2.5", Description: "Moonshot Kimi K2.5", SupportsVision: true},
	{ID: "kimi-k2-0905", Name: "Kimi-K2-0905", Description: "Moonshot Kimi K2 0905", SupportsVision: true},
	{ID: "minimax-m2.5", Name: "MiniMax-M2.5", Description: "MiniMax M2.5", SupportsVision: true},
	{ID: "qwen-vl-max", Name: "Qwen-VL-Max", Description: "通义千问 VL Max 视觉模型", SupportsVision: true},
}

func ConfigureModelParams(body map[string]interface{}, model string) map[string]interface{} {
	configured := cloneMap(body)
	modelLower := strings.ToLower(strings.TrimSpace(model))

	switch {
	case strings.HasPrefix(modelLower, "deepseek"):
		setIfAbsent(configured, "thinking_mode", true)
		setIfAbsent(configured, "reasoning", true)
	case modelLower == "glm-5":
		setIfAbsent(configured, "chat_template_kwargs", map[string]interface{}{"enable_thinking": true})
		setIfAbsent(configured, "enable_thinking", true)
		setIfAbsent(configured, "thinking", map[string]interface{}{"type": "enabled"})
	case modelLower == "glm-4.7":
		setIfAbsent(configured, "chat_template_kwargs", map[string]interface{}{"enable_thinking": true})
	case strings.HasPrefix(modelLower, "glm-"):
		setIfAbsent(configured, "chat_template_kwargs", map[string]interface{}{"enable_thinking": true})
	case strings.HasPrefix(modelLower, "kimi-k2.5"):
		setIfAbsent(configured, "thinking", map[string]interface{}{"type": "enabled"})
	case strings.Contains(modelLower, "thinking"):
		setIfAbsent(configured, "thinking_mode", true)
	case strings.HasPrefix(modelLower, "mimo-"):
		setIfAbsent(configured, "thinking", map[string]interface{}{"type": "enabled"})
	case strings.Contains(modelLower, "claude"):
		setIfAbsent(configured, "chat_template_kwargs", map[string]interface{}{"enable_thinking": true})
	case strings.Contains(modelLower, "sonnet-"):
		setIfAbsent(configured, "chat_template_kwargs", map[string]interface{}{"enable_thinking": true})
	case strings.Contains(modelLower, "reasoning"):
		setIfAbsent(configured, "reasoning", true)
	}

	if qwen4BPattern.MatchString(modelLower) {
		delete(configured, "thinking_mode")
		delete(configured, "reasoning")
		delete(configured, "chat_template_kwargs")
	}

	return configured
}

func NormalizeResponse(response map[string]interface{}, preserveReasoning bool) map[string]interface{} {
	choices, ok := response["choices"].([]interface{})
	if !ok {
		return response
	}

	for _, choice := range choices {
		choiceMap, ok := choice.(map[string]interface{})
		if !ok {
			continue
		}

		message, ok := choiceMap["message"].(map[string]interface{})
		if !ok {
			continue
		}

		content, hasContent := message["content"]
		reasoning, hasReasoning := message["reasoning_content"]
		reasoningText, hasReasoningText := reasoning.(string)
		contentPresent := hasContent && valuePresent(content)

		if preserveReasoning {
			continue
		}

		switch {
		case !contentPresent && hasReasoning && hasReasoningText && reasoningText != "":
			message["content"] = reasoningText
			delete(message, "reasoning_content")
		case contentPresent && hasReasoning:
			delete(message, "reasoning_content")
		}
	}

	return response
}

func setIfAbsent(target map[string]interface{}, key string, value interface{}) {
	if _, exists := target[key]; exists {
		return
	}
	target[key] = value
}

func cloneMap(source map[string]interface{}) map[string]interface{} {
	target := make(map[string]interface{}, len(source))
	for k, v := range source {
		target[k] = v
	}
	return target
}

func valuePresent(value interface{}) bool {
	if value == nil {
		return false
	}
	if s, ok := value.(string); ok {
		return s != ""
	}
	return true
}
