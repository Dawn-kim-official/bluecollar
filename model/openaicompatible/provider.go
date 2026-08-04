package openaicompatible

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Dawn-kim-official/bluecollar/model"
)

type Provider struct {
	endpointURL string
	apiKey      string
	modelName   string
	httpClient  *http.Client
}

func NewProvider(endpointURL string, apiKey string, modelName string) *Provider {
	return &Provider{
		endpointURL: strings.TrimSuffix(strings.TrimSpace(endpointURL), "/"),
		apiKey:      strings.TrimSpace(apiKey),
		modelName:   strings.TrimSpace(modelName),
		httpClient:  http.DefaultClient,
	}
}

func (provider *Provider) UseHTTPClient(httpClient *http.Client) {
	provider.httpClient = httpClient
}

func (provider *Provider) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	response, errorValue := provider.complete(ctx, []model.Message{{Role: "user", Content: prompt}}, nil)
	if errorValue != nil {
		return "", errorValue
	}
	return response.Content, nil
}

func (provider *Provider) GenerateStructuredResponse(ctx context.Context, request model.StructuredResponseRequest) (model.StructuredResponse, error) {
	return provider.complete(ctx, request.Messages, &request.StructuredOutputSchema)
}

func (provider *Provider) complete(ctx context.Context, messages []model.Message, schema *model.StructuredOutputSchema) (model.StructuredResponse, error) {
	body, errorValue := json.Marshal(completionRequest(provider.modelName, messages, schema))
	if errorValue != nil {
		return model.StructuredResponse{}, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpointURL+"/chat/completions", bytes.NewReader(body))
	if errorValue != nil {
		return model.StructuredResponse{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if provider.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+provider.apiKey)
	}

	httpResponse, errorValue := provider.httpClient.Do(httpRequest)
	if errorValue != nil {
		return model.StructuredResponse{}, errorValue
	}
	defer httpResponse.Body.Close()

	responseBody, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return model.StructuredResponse{}, errorValue
	}
	if httpResponse.StatusCode != http.StatusOK {
		return model.StructuredResponse{}, fmt.Errorf("model endpoint returned %d: %s", httpResponse.StatusCode, truncated(string(responseBody)))
	}
	return decodeCompletion(responseBody, provider.modelName)
}

func completionRequest(modelName string, messages []model.Message, schema *model.StructuredOutputSchema) map[string]any {
	request := map[string]any{
		"model":    modelName,
		"messages": chatMessages(messages),
	}
	if schema == nil || strings.TrimSpace(schema.Document) == "" {
		return request
	}
	var schemaDocument any
	if json.Unmarshal([]byte(schema.Document), &schemaDocument) != nil {
		return request
	}
	request["response_format"] = map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   schema.Name,
			"schema": schemaDocument,
			"strict": schema.IsStrictlyEnforced,
		},
	}
	return request
}

func chatMessages(messages []model.Message) []map[string]string {
	chat := make([]map[string]string, 0, len(messages))
	for _, message := range messages {
		content := message.Content
		for _, part := range message.Parts {
			if part.Text != "" {
				content += part.Text
			}
		}
		chat = append(chat, map[string]string{"role": message.Role, "content": content})
	}
	return chat
}

func decodeCompletion(responseBody []byte, modelName string) (model.StructuredResponse, error) {
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if errorValue := json.Unmarshal(responseBody, &decoded); errorValue != nil {
		return model.StructuredResponse{}, errorValue
	}
	if len(decoded.Choices) == 0 {
		return model.StructuredResponse{}, errors.New("model endpoint returned no choices")
	}
	return model.StructuredResponse{
		Transport:    "http",
		ProviderName: "openai-compatible",
		ModelName:    modelName,
		Content:      decoded.Choices[0].Message.Content,
		FinishReason: decoded.Choices[0].FinishReason,
		Usage: model.Usage{
			PromptTokens:     decoded.Usage.PromptTokens,
			CompletionTokens: decoded.Usage.CompletionTokens,
			TotalTokens:      decoded.Usage.TotalTokens,
		},
	}, nil
}

func truncated(text string) string {
	const limit = 300
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}
