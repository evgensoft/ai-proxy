package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"ai-proxy/internal/schema"
)

func CreateRequest(providerURL, model, token string, reqBody schema.RequestOpenAICompatable) (*http.Request, error) {
	reqBody.Model = model

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, providerURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	return req, nil
}

func Call(providerURL, model, token string, reqBody schema.RequestOpenAICompatable) ([]byte, error) {
	req, err := CreateRequest(providerURL, model, token, reqBody)
	if err != nil {
		return nil, err
	}

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

	return body, nil
}
