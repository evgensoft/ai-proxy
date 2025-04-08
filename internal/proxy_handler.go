package internal

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"ai-proxy/internal/gemini"
	"ai-proxy/internal/gigachat"
	"ai-proxy/internal/openai"
	"ai-proxy/internal/schema"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type RateLimit struct {
	minuteCount int
	hourCount   int
	dayCount    int
	lastMinute  time.Time
	lastHour    time.Time
	lastDay     time.Time
	lastRequest time.Time
	mux         sync.Mutex // Fine-grained locking for each rate limit
}

type Model struct {
	Name             string `yaml:"name"`
	Provider         string `yaml:"provider"`
	Priority         int    `yaml:"priority"`
	RequestsPerMin   int    `yaml:"requests_per_minute"`
	RequestsPerHour  int    `yaml:"requests_per_hour"`
	RequestsPerDay   int    `yaml:"requests_per_day"`
	URL              string `yaml:"url"`
	Token            string `yaml:"token"`
	MaxRequestLength int    `yaml:"max_request_length"`
	Size             string `yaml:"model_size"`
}

var (
	RateLimits map[string]*RateLimit
	Models     []Model
)

func HandlerTxt(w http.ResponseWriter, req *http.Request) {
	var (
		modelName, modelSize string
		response             []byte
		err                  error
	)

	if req.Method != http.MethodPost {
		http.Error(w, "", http.StatusServiceUnavailable)

		return
	}

	reqBodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	if !gjson.ValidBytes(reqBodyBytes) {
		http.Error(w, "Invalid request body", http.StatusBadRequest)

		return
	}

	modelName = gjson.GetBytes(reqBodyBytes, "model").String()

	if len(modelName) < 10 {
		if modelName != "BIG" {
			modelSize = "SMALL"
		} else {
			modelSize = "BIG"
		}

		for i := 0; i < 5; i++ {
			modelName = selectModel(modelSize, len(reqBodyBytes))
			if modelName == "" {
				log.Printf("No available models for this request length = %d", len(reqBodyBytes))
				http.Error(w, "No available models for this request length", http.StatusServiceUnavailable)

				return
			}

			response, err = sendRequestToLLM(modelName, reqBodyBytes)
			if err != nil {
				setMaxLimitMinute(modelName) // set max minuteCount for pause after error
				log.Printf("Error sending request to LLM: %v", err)

				continue
			}

			break
		}
	} else {
		response, err = sendRequestToLLM(modelName, reqBodyBytes)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func setMaxLimitMinute(modelName string) {
	var model Model

	for _, m := range Models {
		if m.Name == modelName {
			model = m

			break
		}
	}

	limit := RateLimits[modelName]

	limit.mux.Lock()
	defer limit.mux.Unlock()

	limit.minuteCount = model.RequestsPerMin + 1
	limit.lastMinute = time.Now()
}

func selectModel(modelSize string, requestLength int) string {
	var selectedModel *Model

	var selectedLastRequest time.Time

	for _, model := range Models {
		if model.Size != modelSize {
			continue
		}

		if requestLength > model.MaxRequestLength {
			continue
		}

		limit := RateLimits[model.Name]
		limit.mux.Lock() // Lock individual rate limit

		if limit.minuteCount < model.RequestsPerMin &&
			limit.hourCount < model.RequestsPerHour &&
			limit.dayCount < model.RequestsPerDay {
			// Select the model with the lowest priority
			// If priorities are equal, select the one with the earliest lastRequest
			if selectedModel == nil {
				selectedModel = &model
				selectedLastRequest = limit.lastRequest
			} else if model.Priority < selectedModel.Priority {
				selectedModel = &model
				selectedLastRequest = limit.lastRequest
			} else if model.Priority == selectedModel.Priority {
				// && limit.lastRequest.Before(selectedLastRequest) {
				if limit.lastRequest.Before(time.Now().Add(-time.Hour)) && selectedLastRequest.Before(time.Now().Add(-time.Hour)) &&
					model.MaxRequestLength < selectedModel.MaxRequestLength {
					selectedModel = &model
					selectedLastRequest = limit.lastRequest
				} else if limit.lastRequest.Before(selectedLastRequest) {
					selectedModel = &model
					selectedLastRequest = limit.lastRequest
				}
			}
		}

		limit.mux.Unlock()
	}

	if selectedModel == nil {
		return ""
	}

	return selectedModel.Name
}

func updateLimitCounters(limit *RateLimit, now time.Time) {
	if now.Sub(limit.lastMinute) >= time.Minute {
		limit.minuteCount = 0
		limit.lastMinute = now
	}

	if now.Sub(limit.lastHour) >= time.Hour {
		limit.hourCount = 0
		limit.lastHour = now
	}

	if now.Sub(limit.lastDay) >= 24*time.Hour {
		limit.dayCount = 0
		limit.lastDay = now
	}
}

func incrementRateLimit(modelName string) {
	limit := RateLimits[modelName]

	limit.mux.Lock()
	defer limit.mux.Unlock()

	now := time.Now()

	updateLimitCounters(limit, now)

	limit.minuteCount++
	limit.hourCount++
	limit.dayCount++
	limit.lastRequest = now
}

func getModelByName(modelName string) (Model, bool) {
	for _, model := range Models {
		if model.Name == modelName {
			incrementRateLimit(modelName)

			return model, true
		}
	}

	return Model{}, false
}

func getRequestLength(requestBody schema.RequestOpenAICompatable) int {
	var res int

	for _, v := range requestBody.Messages {
		res += len(v.Content)
	}

	return res
}

// func sendRequestToLLM(modelName string, requestBody schema.RequestOpenAICompatable) ([]byte, error) {
func sendRequestToLLM(modelName string, requestBody []byte) ([]byte, error) {
	var resp []byte

	var err error

	log.Printf("Request to model: %s - %s\n", modelName, printFirstChars(gjson.GetBytes(requestBody, "messages.0.content").String()))

	model, found := getModelByName(modelName)
	if !found {
		return nil, fmt.Errorf("Specified model not found - %s", modelName)
	}

	switch model.Provider {
	case "cloudflare":
		resp, err = openai.Call(model.URL, "@"+model.Name, model.Token, requestBody)
	case "google": // todo change on openai.Call - https://developers.googleblog.com/en/gemini-is-now-accessible-from-the-openai-library/
		resp, err = gemini.Call(model.URL, model.Name, model.Token, requestBody)
	case "gigachat":
		resp, err = gigachat.Call(model.URL, strings.TrimPrefix(model.Name, model.Provider+"/"), model.Token, requestBody)
	case "groq", "arliai", "github":
		resp, err = openai.Call(model.URL, strings.TrimPrefix(model.Name, model.Provider+"/"), model.Token, requestBody)
	case "cohere":
		resp, err = openai.Call(model.URL, strings.TrimPrefix(model.Name, model.Provider+"/"), model.Token, requestBody)
		if err == nil {
			response := schema.ResponseOpenAICompatable{
				Model: model.Name,
				Choices: []struct {
					Index   int `json:"index,omitempty"`
					Message struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
					} `json:"message,omitempty"`
					FinishReason string `json:"finish_reason,omitempty"`
				}{
					{
						Index: 0,
						Message: struct {
							Role    string `json:"role,omitempty"`
							Content string `json:"content,omitempty"`
						}{
							Role:    "assistant",
							Content: gjson.GetBytes(resp, "message.content.0.text").String(),
						},
						FinishReason: "stop",
					},
				},
			}
			resp, err = json.Marshal(response)
		}
	default:
		resp, err = openai.Call(model.URL, strings.TrimPrefix(model.Name, model.Provider+"/"), model.Token, requestBody)
	}

	if err != nil {
		log.Printf("ERROR: %s, body: %s\n", err, string(resp))

		return nil, err
	}

	if resp == nil {
		return nil, fmt.Errorf("No response from LLM")
	}

	content := gjson.GetBytes(resp, "choices.0.message.content").String()
	log.Printf("Response: %s\n", printFirstChars(cmp.Or(content, string(resp))))

	if len(content) == 0 {
		return nil, fmt.Errorf("no content")
	}

	// replace block </think> in DeepSeek-R1
	if strings.Contains(model.Name, "DeepSeek-R1") {
		thinkTag := "</think>"

		index := strings.Index(content, thinkTag)
		if index != -1 {
			content = content[index+len(thinkTag):]

			resp, err = sjson.SetBytes(resp, "choices.0.message.content", content)
			if err != nil {
				return nil, fmt.Errorf("error sjson.SetBytes in replace think: %w", err)
			}
		}
	}

	return resp, nil
}

func printFirstChars(data string) string {
	if len(data) > 100 {
		return strings.TrimSpace(data[:100])
	}

	return data
}
