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

	got := ConfigureModelParams(body, "glm-5")

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

	got := ConfigureModelParams(body, "qwen2.5-4b-instruct")

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
	got := ConfigureModelParams(map[string]interface{}{}, "kimi-k2-thinking")
	if got["thinking_mode"] != true {
		t.Fatalf("thinking_mode = %v, want true", got["thinking_mode"])
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
