package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"ai-proxy/internal/cloudflare"
	"ai-proxy/internal/gemini"
	"ai-proxy/internal/groq"
	"ai-proxy/internal/schema"
)

type Provider struct {
	lastTime time.Time
	Name     string
	URL      string
	Token    string
	Model    string
}

// init map
var ProvidersMap = map[string]Provider{
	// https://console.groq.com/settings/limits
	// models -
	// llama3-70b-8192
	// llama3-8b-8192
	// gemma-7b-it
	// mixtral-8x7b-32768
	"/groq/chat/completions": {
		Name:  "Groq",
		URL:   "https://gateway.ai.cloudflare.com/v1/303130b3ee2cdf55b28c1da7b2b6b6c5/ai-gateway/groq/chat/completions",
		Model: "llama-3.1-70b-versatile",
		Token: os.Getenv("GROQ_TOKEN"),
	},
	// list models - https://developers.cloudflare.com/workers-ai/models/#text-generation
	// beta models is free!
	"/cloudflare/chat/completions": {
		Name:  "Cloudflare",
		URL:   "https://gateway.ai.cloudflare.com/v1/303130b3ee2cdf55b28c1da7b2b6b6c5/ai-gateway/workers-ai/v1/chat/completions",
		Model: "@cf/meta/llama-3-8b-instruct",
		Token: os.Getenv("CLOUDFLARE_TOKEN"),
	},
	// request by proxy USA
	// models - https://ai.google.dev/gemini-api/docs/models/gemini?hl=ru
	// gemini-pro
	// gemini-1.5-pro
	// gemini-1.5-flash
	"/gemini/chat/completions": {
		Name:  "Google",
		URL:   "https://generativelanguage.googleapis.com/v1/models",
		Model: "gemini-1.5-flash",
		Token: os.Getenv("GEMINI_TOKEN"),
	},
}

func Req(w http.ResponseWriter, req *http.Request) {
	provider, ok := ProvidersMap[req.RequestURI]
	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)

		return
	}

	var reqBody schema.RequestOpenAICompatable

	err := json.NewDecoder(req.Body).Decode(&reqBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	if reqBody.Model == "" {
		reqBody.Model = provider.Model
	}

	var resp []byte

	switch provider.Name {
	case "Cloudflare":
		resp, err = cloudflare.Call(provider.URL, provider.Model, provider.Token, reqBody)
	case "Google":
		resp, err = gemini.Call(provider.URL, provider.Model, provider.Token, reqBody)
	case "Groq":
		resp, err = groq.Call(provider.URL, provider.Model, provider.Token, reqBody)

	default:
		err = fmt.Errorf("provider %s not found", provider.Name)
	}

	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Write(resp)

	return
}
