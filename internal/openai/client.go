package openai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/tidwall/sjson"
)

func Call(providerURL, model, token string, requestBody []byte) ([]byte, error) {
	reqBody, err := sjson.SetBytes(requestBody, "model", model)
	if err != nil {
		return nil, fmt.Errorf("error in sjson.SetBytes: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, providerURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

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
