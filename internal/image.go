package internal

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"
)

// Пример запроса
// curl https://ai-proxy-evgensoft.koyeb.app/image -d '{"model": "huggingface/black-forest-labs/FLUX.1-dev", "prompt": "Cat sleep on the moon"}'

type RequestGenerateImage struct {
	Model  string `json:"model,omitempty"`
	Prompt string `json:"prompt,omitempty"`
	Inputs string `json:"inputs,omitempty"`
}

type TogetherRequestGenerateImage struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	ResponseFormat string `json:"response_format"`
}

func HandlerImage(w http.ResponseWriter, req *http.Request) {
	var (
		requestBody RequestGenerateImage
		response    []byte
		err         error
	)

	err = json.NewDecoder(req.Body).Decode(&requestBody)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)

		return
	}

	prompt := cmp.Or(requestBody.Prompt, requestBody.Inputs)

	if len(prompt) == 0 {
		http.Error(w, "Empty prompt", http.StatusBadRequest)

		return
	}

	if requestBody.Model == "" || requestBody.Model == "all" {
		for _, model := range Models {
			if model.MaxRequestLength != 0 {
				continue
			}

			if RateLimits[model.Name].minuteCount >= model.RequestsPerMin ||
				RateLimits[model.Name].hourCount >= model.RequestsPerHour ||
				RateLimits[model.Name].dayCount >= model.RequestsPerDay {
				continue
			}

			response, err = RequestProvider(model.Name, prompt)
			if err != nil {
				setMaxLimitMinute(model.Name) // set max minuteCount for pause after error
				log.Printf("ERROR: %s, body: %s\n", err, string(response))

				continue
			}

			break
		}
	} else {
		response, err = RequestProvider(requestBody.Model, prompt)
	}

	if err != nil {
		log.Printf("ERROR: %s, body: %s\n", err, string(response))
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	log.Printf("Get image in %d bytes\n", len(response))

	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(response)
}

func RequestProvider(modelName, prompt string) ([]byte, error) {
	model, found := getModelByName(modelName)
	if !found {
		return nil, fmt.Errorf("Specified model not found - %s", modelName)
	}

	log.Printf("Request to image model: %s - %s\n", modelName, printFirstChars(prompt))

	if model.Provider == "airforce" {
		return getAairforceImagine(model.URL, prompt, strings.TrimPrefix(model.Name, model.Provider+"/"))
	}

	data, err := generatePayload(model, prompt)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, model.URL, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", model.Token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if len(body) < 500 {
		return body, fmt.Errorf("small response length: %d", len(body))
	}

	switch model.Name {
	case "cloudflare/black-forest-labs/flux-1-schnell":
		return base64.StdEncoding.DecodeString(gjson.GetBytes(body, "result.image").String())
	case "together/black-forest-labs/FLUX.1-schnell-Free", "aimlapi/flux/schnell":
		return base64.StdEncoding.DecodeString(gjson.GetBytes(body, "data.0.b64_json").String())

	default:
		return body, nil
	}
}

func generatePayload(model Model, prompt string) ([]byte, error) {
	var data []byte

	var err error

	switch model.Provider {
	case "huggingface":
		var payload RequestGenerateImage

		payload.Inputs = prompt

		data, err = json.Marshal(payload)

	// case "cloudflare":

	case "together", "aimlapi":
		var payload TogetherRequestGenerateImage

		payload.Model = strings.TrimPrefix(model.Name, model.Provider+"/")
		payload.Prompt = prompt
		payload.ResponseFormat = "b64_json"

		data, err = json.Marshal(payload)

	default:
		var payload RequestGenerateImage

		payload.Prompt = prompt

		data, err = json.Marshal(payload)
	}

	return data, err
}

func getAairforceImagine(baseURL, prompt, model string) ([]byte, error) {
	// Формируем URL с параметрами
	params := url.Values{}
	params.Add("prompt", prompt)
	params.Add("model", model)

	// Создаем полный URL
	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// Выполняем GET запрос
	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if len(body) < 500 {
		return body, fmt.Errorf("small response length: %d", len(body))
	}

	return body, nil
}
