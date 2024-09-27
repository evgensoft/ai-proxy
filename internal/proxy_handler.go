package internal

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"ai-proxy/internal/gemini"
	"ai-proxy/internal/openai"
	"ai-proxy/internal/schema"
)

type RateLimit struct {
	minuteCount int
	hourCount   int
	dayCount    int
	lastMinute  time.Time
	lastHour    time.Time
	lastDay     time.Time
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
}

type Config struct {
	Models []Model `yaml:"models"`
}
type ProxyHandler struct {
	config     Config
	rateLimits map[string]*RateLimit
	mu         sync.Mutex
}

func NewProxyHandler(config Config) (*ProxyHandler, error) {
	rateLimits := make(map[string]*RateLimit)
	for _, model := range config.Models {
		rateLimits[model.Name] = &RateLimit{
			lastMinute: time.Now(),
			lastHour:   time.Now(),
			lastDay:    time.Now(),
		}
	}

	return &ProxyHandler{
		config:     config,
		rateLimits: rateLimits,
	}, nil
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		requestBody schema.RequestOpenAICompatable
		response    []byte
		err         error
	)

	log.Printf("Request: %s %s\n", r.Method, r.URL.Path)

	err = json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)

		return
	}

	log.Printf("Request: %s\n", printFirstChars(requestBody.Messages[0].Content))

	if requestBody.Model == "" {
		for range 3 {
			requestBody.Model = h.selectModel(calculateRequestLength(requestBody))
			if requestBody.Model == "" {
				http.Error(w, "No available models for this request length", http.StatusServiceUnavailable)

				return
			}

			response, err = h.sendRequestToLLM(requestBody.Model, requestBody)
			if err != nil {
				log.Printf("Error sending request to LLM: %v", err)

				continue
			}

			break
		}
	} else {
		response, err = h.sendRequestToLLM(requestBody.Model, requestBody)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}
	}

	w.Write(response)
}

func (h *ProxyHandler) selectModel(requestLength int) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	availableModels := make([]Model, 0)

	for _, model := range h.config.Models {
		limit := h.rateLimits[model.Name]

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

		if limit.minuteCount < model.RequestsPerMin &&
			limit.dayCount < model.RequestsPerDay &&
			requestLength <= model.MaxRequestLength {
			availableModels = append(availableModels, model)
		}
	}

	if len(availableModels) == 0 {
		return ""
	}

	sort.Slice(availableModels, func(i, j int) bool {
		// First sort by Priority
		if availableModels[i].Priority != availableModels[j].Priority {
			return availableModels[i].Priority < availableModels[j].Priority
		}
		// If Priority is the same, sort by lastMinute
		return h.rateLimits[availableModels[i].Name].minuteCount < h.rateLimits[availableModels[j].Name].minuteCount
	})

	selectedModel := availableModels[0]

	limit := h.rateLimits[selectedModel.Name]
	limit.minuteCount++
	limit.hourCount++
	limit.dayCount++

	return selectedModel.Name
}

func (h *ProxyHandler) getModelByName(name string) (Model, bool) {
	for _, model := range h.config.Models {
		if model.Name == name {
			return model, true
		}
	}

	return Model{}, false
}

func calculateRequestLength(requestBody schema.RequestOpenAICompatable) int {
	var res int

	for _, v := range requestBody.Messages {
		res += len(v.Content)
	}

	return res
}

func (h *ProxyHandler) sendRequestToLLM(modelName string, requestBody schema.RequestOpenAICompatable) ([]byte, error) {
	var resp []byte

	var err error

	fmt.Printf("Request to model: %s - %s\n", modelName, printFirstChars(requestBody.Messages[0].Content))

	model, found := h.getModelByName(modelName)
	if !found {
		return nil, fmt.Errorf("Specified model not found - %s", modelName)
	}

	switch model.Provider {
	case "Cloudflare":
		resp, err = openai.Call(model.URL, "@"+model.Name, model.Token, requestBody)
	case "Google":
		resp, err = gemini.Call(model.URL, model.Name, model.Token, requestBody)
	case "groq", "arliai", "github":
		resp, err = openai.Call(model.URL, strings.TrimPrefix(model.Name, model.Provider+"/"), model.Token, requestBody)

	default:
		resp, err = openai.Call(model.URL, strings.TrimPrefix(model.Name, model.Provider+"/"), model.Token, requestBody)
	}

	if err != nil {
		fmt.Printf("ERROR: %s, body: %s\n", err, string(resp))

		return nil, err
	}

	fmt.Printf("Response: %s\n", printFirstChars(string(resp)))

	return resp, nil
}

func printFirstChars(data string) string {
	if len(data) > 100 {
		return data[:100]
	}

	return data
}
