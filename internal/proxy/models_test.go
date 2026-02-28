package proxy

import "testing"

func TestModelsContainsKnownEntries(t *testing.T) {
	var hasGLM5 bool
	var hasQwenVL bool

	for _, m := range Models {
		if m.ID == "glm-5" {
			hasGLM5 = true
		}
		if m.ID == "qwen-vl-max" && m.SupportsVision {
			hasQwenVL = true
		}
	}

	if !hasGLM5 {
		t.Fatal("Models missing glm-5")
	}
	if !hasQwenVL {
		t.Fatal("Models missing qwen-vl-max with SupportsVision=true")
	}
}

func TestConfigureModelParamsGLM5(t *testing.T) {
	body := map[string]interface{}{
		"model": "glm-5",
	}

	got := ConfigureModelParams(body, "glm-5", "https://apis.iflow.cn/v1", "session-1")

	if got["enable_thinking"] != true {
		t.Fatalf("enable_thinking = %v, want true", got["enable_thinking"])
	}

	thinking, ok := got["thinking"].(map[string]interface{})
	if !ok || thinking["type"] != "enabled" {
		t.Fatalf("thinking = %#v, want {type: enabled}", got["thinking"])
	}

	chatTemplate, ok := got["chat_template_kwargs"].(map[string]interface{})
	if !ok || chatTemplate["enable_thinking"] != true {
		t.Fatalf("chat_template_kwargs = %#v, want {enable_thinking: true}", got["chat_template_kwargs"])
	}

	if _, exists := body["enable_thinking"]; exists {
		t.Fatal("input body should not be mutated")
	}
}

func TestConfigureModelParamsQwen4BRemovesThinkingFields(t *testing.T) {
	body := map[string]interface{}{
		"thinking_mode":        true,
		"reasoning":            true,
		"chat_template_kwargs": map[string]interface{}{"enable_thinking": true},
	}

	got := ConfigureModelParams(body, "qwen2.5-4b-instruct", "https://apis.iflow.cn/v1", "session-1")

	if _, ok := got["thinking_mode"]; ok {
		t.Fatalf("thinking_mode should be removed, got %#v", got["thinking_mode"])
	}
	if _, ok := got["reasoning"]; ok {
		t.Fatalf("reasoning should be removed, got %#v", got["reasoning"])
	}
	if _, ok := got["chat_template_kwargs"]; ok {
		t.Fatalf("chat_template_kwargs should be removed, got %#v", got["chat_template_kwargs"])
	}
}

func TestConfigureModelParamsThinkingBranch(t *testing.T) {
	got := ConfigureModelParams(map[string]interface{}{}, "kimi-k2-thinking", "https://apis.iflow.cn/v1", "session-1")
	if got["thinking_mode"] != true {
		t.Fatalf("thinking_mode = %v, want true", got["thinking_mode"])
	}
}

func TestConfigureModelParamsGLM46MapsToExpAndDefaults(t *testing.T) {
	got := ConfigureModelParams(map[string]interface{}{}, "glm-4.6", "https://apis.iflow.cn/v1", "session-1")
	if got["model"] != "glm-4.6-exp" {
		t.Fatalf("model = %#v, want glm-4.6-exp", got["model"])
	}
	if got["temperature"] != 0.6 {
		t.Fatalf("temperature = %#v, want 0.6", got["temperature"])
	}
	if got["max_new_tokens"] != 32000 {
		t.Fatalf("max_new_tokens = %#v, want 32000", got["max_new_tokens"])
	}
}

func TestConfigureModelParamsConvertsMaxTokens(t *testing.T) {
	got := ConfigureModelParams(map[string]interface{}{"max_tokens": 256}, "glm-4.7", "https://apis.iflow.cn/v1", "session-1")
	if _, ok := got["max_tokens"]; ok {
		t.Fatalf("max_tokens should be removed, got %#v", got["max_tokens"])
	}
	if got["max_new_tokens"] != 256 {
		t.Fatalf("max_new_tokens = %#v, want 256", got["max_new_tokens"])
	}
}

func TestConfigureModelParamsROMEOverridesSampling(t *testing.T) {
	got := ConfigureModelParams(map[string]interface{}{
		"temperature": 0.1,
		"top_p":       0.2,
		"top_k":       5,
	}, "iFlow-ROME-30BA3B", "https://apis.iflow.cn/v1", "session-1")

	if got["temperature"] != 0.7 {
		t.Fatalf("temperature = %#v, want 0.7", got["temperature"])
	}
	if got["top_p"] != 0.8 {
		t.Fatalf("top_p = %#v, want 0.8", got["top_p"])
	}
	if got["top_k"] != 20 {
		t.Fatalf("top_k = %#v, want 20", got["top_k"])
	}
}

func TestConfigureModelParamsWhaleWaveInjectsExtendFields(t *testing.T) {
	got := ConfigureModelParams(map[string]interface{}{}, "glm-4.6", "https://api.whale-wave.example/v1", "session-xyz")
	extend, ok := got["extend_fields"].(map[string]interface{})
	if !ok {
		t.Fatalf("extend_fields = %#v, want map", got["extend_fields"])
	}
	if extend["sessionId"] != "session-xyz" {
		t.Fatalf("extend_fields.sessionId = %#v, want session-xyz", extend["sessionId"])
	}
}

func TestNormalizeResponseMovesReasoningContent(t *testing.T) {
	response := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"content":           nil,
					"reasoning_content": "step-by-step",
				},
			},
		},
	}

	got := NormalizeResponse(response, false)
	choices := got["choices"].([]interface{})
	msg := choices[0].(map[string]interface{})["message"].(map[string]interface{})

	if msg["content"] != "step-by-step" {
		t.Fatalf("content = %#v, want step-by-step", msg["content"])
	}
	if _, ok := msg["reasoning_content"]; ok {
		t.Fatalf("reasoning_content should be removed, got %#v", msg["reasoning_content"])
	}
}

func TestNormalizeResponsePreservesReasoningWhenRequested(t *testing.T) {
	response := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"content":           "final",
					"reasoning_content": "reasoning",
				},
			},
		},
	}

	got := NormalizeResponse(response, true)
	choices := got["choices"].([]interface{})
	msg := choices[0].(map[string]interface{})["message"].(map[string]interface{})

	if msg["content"] != "final" {
		t.Fatalf("content = %#v, want final", msg["content"])
	}
	if msg["reasoning_content"] != "reasoning" {
		t.Fatalf("reasoning_content = %#v, want reasoning", msg["reasoning_content"])
	}
}

func TestNormalizeResponsePreserveDoesNotMirrorReasoningToContent(t *testing.T) {
	response := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"content":           nil,
					"reasoning_content": "step-by-step",
				},
			},
		},
	}

	got := NormalizeResponse(response, true)
	choices := got["choices"].([]interface{})
	msg := choices[0].(map[string]interface{})["message"].(map[string]interface{})

	if msg["content"] != nil {
		t.Fatalf("content = %#v, want nil", msg["content"])
	}
	if msg["reasoning_content"] != "step-by-step" {
		t.Fatalf("reasoning_content = %#v, want step-by-step", msg["reasoning_content"])
	}
}

func TestNormalizeResponseDeletesReasoningWhenNotPreserved(t *testing.T) {
	response := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"content":           "final",
					"reasoning_content": "reasoning",
				},
			},
		},
	}

	got := NormalizeResponse(response, false)
	choices := got["choices"].([]interface{})
	msg := choices[0].(map[string]interface{})["message"].(map[string]interface{})

	if msg["content"] != "final" {
		t.Fatalf("content = %#v, want final", msg["content"])
	}
	if _, ok := msg["reasoning_content"]; ok {
		t.Fatalf("reasoning_content should be removed, got %#v", msg["reasoning_content"])
	}
}
