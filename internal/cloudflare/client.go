package cloudflare

import (
	"ai-proxy/internal/schema"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

var (
	queue           = make(chan interface{}, 1)
	lastTimeRequest = time.Now()

	maxTimeoutTime = 1 * time.Second
)

func CreateRequest(providerURL, model, token string, reqBody schema.RequestOpenAICompatable) (*http.Request, error) {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	// fmt.Printf("JSON BODY: %s\n", string(jsonBody))

	req, err := http.NewRequest(http.MethodPost, providerURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	return req, nil
}

func Call(providerURL, model, token string, reqBody schema.RequestOpenAICompatable) ([]byte, error) {
	// Выполняем запросы в 1 поток
	queue <- struct{}{}
	defer func() {
		// Запоминаем время последнего запроса и очищаем очередь
		lastTimeRequest = time.Now()

		<-queue
	}()

	if !time.Now().After(lastTimeRequest.Add(maxTimeoutTime)) {
		log.Printf("Throttled %v seс for %s", time.Until(lastTimeRequest.Add(maxTimeoutTime)).Seconds(), model)
		time.Sleep(time.Until(lastTimeRequest.Add(maxTimeoutTime)))
	}

	req, err := CreateRequest(providerURL, model, token, reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
