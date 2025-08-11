package gigachat

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// --- Структуры для парсинга ответа GigaChat ---

type GigaChatResponse struct {
	Choices []GigaChatChoice `json:"choices"`
	Created int64            `json:"created"` // Unix_timestamp в секундах
	Model   string           `json:"model"`
	Usage   GigaChatUsage    `json:"usage"`
	Object  string           `json:"object"` // "completion" или "chat.completion"
}

type GigaChatChoice struct {
	Message      GigaChatMessage `json:"message"`
	Index        int             `json:"index"`
	FinishReason string          `json:"finish_reason"`
}

type GigaChatMessage struct {
	Content      string                `json:"content"` // Может быть пустой строкой
	Role         string                `json:"role"`
	FunctionCall *GigaChatFunctionCall `json:"function_call,omitempty"`
	// functions_state_id игнорируем
}

type GigaChatFunctionCall struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"` // Аргументы могут быть сложным JSON объектом/массивом
}

type GigaChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	SystemTokens     int `json:"system_tokens,omitempty"` // Добавлено согласно API GigaChat
}

// --- Структуры для формирования ответа в формате OpenAI ---

type OpenAIResponse struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"` // Unix timestamp в секундах
	Model             string         `json:"model"`
	Choices           []OpenAIChoice `json:"choices"`
	Usage             OpenAIUsage    `json:"usage"` // Не указатель, т.к. документация GigaChat предполагает его наличие
	SystemFingerprint string         `json:"system_fingerprint"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	Logprobs     interface{}   `json:"logprobs"`      // Всегда null в Python-примере
	FinishReason string        `json:"finish_reason"` // Копируем из GigaChat
}

type OpenAIMessage struct {
	Role         string              `json:"role"`
	Content      *string             `json:"content"`           // Указатель для поддержки null. Не omitempty, чтобы null сериализовался.
	Refusal      interface{}         `json:"refusal,omitempty"` // Всегда null в Python-примере и omitempty
	FunctionCall *OpenAIFunctionCall `json:"function_call,omitempty"`
	ToolCalls    []OpenAIToolCall    `json:"tool_calls,omitempty"`
}

type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // Аргументы должны быть строкой JSON
}

type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"` // "function"
	Function OpenAIFunctionCall `json:"function"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	SystemTokens     int `json:"system_tokens,omitempty"` // Добавлено для соответствия Python-прокси
}

// ConvertGigaChatResponseToOpenAI преобразует ответ GigaChat в формат OpenAI Chat Completion.
// gigaChatResponseBytes: сырой JSON ответ от GigaChat API.
// openAIModelName: имя модели, которое будет указано в поле 'model' ответа OpenAI.
// isToolCall: если true, преобразует function_call в tool_calls.
func ConvertGigaChatResponseToOpenAI(gigaChatResponseBytes []byte, openAIModelName string, isToolCall bool) ([]byte, error) {
	var gigaResp GigaChatResponse

	err := json.Unmarshal(gigaChatResponseBytes, &gigaResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal GigaChat response: %w", err)
	}

	// Генерируем UUID для system_fingerprint
	u := uuid.New()
	uuidBytes := u[:]
	shortHexID := hex.EncodeToString(uuidBytes)[:8]

	openAIResp := OpenAIResponse{
		ID:                fmt.Sprintf("chatcmpl-%s", uuid.NewString()),
		Object:            "chat.completion", // Для не-стриминга, как в Python
		Created:           time.Now().Unix(), // Unix timestamp в секундах
		Model:             openAIModelName,
		SystemFingerprint: fmt.Sprintf("fp_%s", shortHexID),
		Usage: OpenAIUsage{ // Заполняем напрямую, т.к. ожидаем, что GigaChat всегда возвращает usage
			PromptTokens:     gigaResp.Usage.PromptTokens,
			CompletionTokens: gigaResp.Usage.CompletionTokens,
			TotalTokens:      gigaResp.Usage.TotalTokens,
			SystemTokens:     gigaResp.Usage.SystemTokens, // Включаем, следуя Python-коду
		},
		Choices: make([]OpenAIChoice, 0, len(gigaResp.Choices)),
	}

	for _, gigaChoice := range gigaResp.Choices {
		openAIChoice := OpenAIChoice{
			Index:        0,   // Хардкод из Python-кода
			Logprobs:     nil, // Хардкод из Python-кода
			FinishReason: gigaChoice.FinishReason,
			Message: OpenAIMessage{
				Role:    gigaChoice.Message.Role,
				Refusal: nil, // Хардкод из Python-кода, с omitempty будет отсутствовать
			},
		}

		// Обработка Content: делаем nil, если пустой И есть function call
		// Python: if choice["message"].get("content") == "" and choice["message"].get("function_call"):
		//             choice["message"]["content"] = None
		// В Go: gigaChoice.Message.Content будет "" если GigaChat прислал "" или не прислал content вообще (для строкового типа)
		if gigaChoice.Message.Content == "" && gigaChoice.Message.FunctionCall != nil {
			openAIChoice.Message.Content = nil
		} else {
			// Копируем значение, чтобы получить указатель.
			// Если gigaChoice.Message.Content пустая строка, но FunctionCall нет, то content будет "" (не null)
			content := gigaChoice.Message.Content
			openAIChoice.Message.Content = &content
		}

		// Обработка FunctionCall / ToolCalls
		// Python: if choice["message"]["role"] == "assistant" and choice["message"].get("function_call"):
		if gigaChoice.Message.Role == "assistant" && gigaChoice.Message.FunctionCall != nil {
			// Сериализуем аргументы в строку JSON
			// Python: arguments = json.dumps(..., ensure_ascii=False)
			// Go: json.Marshal по умолчанию сохраняет UTF-8 символы (аналогично ensure_ascii=False)
			argsBytes, err := json.Marshal(gigaChoice.Message.FunctionCall.Arguments)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal function call arguments for choice %d: %w", gigaChoice.Index, err)
			}

			argsString := string(argsBytes)

			oaFuncCall := OpenAIFunctionCall{
				Name:      gigaChoice.Message.FunctionCall.Name,
				Arguments: argsString,
			}

			if isToolCall {
				// Python: choice["message"]["tool_calls"] = [{"id": ..., "type": "function", "function": choice["message"].pop("function_call")}]
				toolCall := OpenAIToolCall{
					ID:       fmt.Sprintf("call_%s", uuid.NewString()), // Генерируем ID для tool_call
					Type:     "function",
					Function: oaFuncCall,
				}
				openAIChoice.Message.ToolCalls = append(openAIChoice.Message.ToolCalls, toolCall)
				openAIChoice.Message.FunctionCall = nil // Явно не устанавливаем, будет omitempty
			} else {
				// Python: choice["message"].pop("tool_calls", None)
				openAIChoice.Message.FunctionCall = &oaFuncCall
				openAIChoice.Message.ToolCalls = nil // Явно не устанавливаем, будет omitempty
			}
		}

		openAIResp.Choices = append(openAIResp.Choices, openAIChoice)
	}

	openAIResponseBytes, err := json.Marshal(openAIResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI response: %w", err)
	}

	return openAIResponseBytes, nil
}
