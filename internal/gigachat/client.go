package gigachat

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/evgensoft/gigachat"
	"github.com/tidwall/sjson"
)

var (
	queue           = make(chan interface{}, 1)
	lastTimeRequest = time.Now()

	maxTimeoutTime = 1 * time.Second
	gigachatClient *gigachat.Client
)

func InitClient(token string) error {
	// Разбиваем строку на clientID и clientSecret
	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("error in strings.SplitN: %v parts", len(parts))
	}

	clientID := parts[0]
	clientSecret := parts[1]

	// Создаем клиент (по умолчанию используется ScopePersonal)
	gigachatClient = gigachat.NewClient(clientID, clientSecret)

	// При необходимости можно изменить scope
	gigachatClient.SetScope(gigachat.ScopeCorp)

	return nil
}

func Call(providerURL, model, token string, requestBody []byte) ([]byte, error) {
	// Выполняем запросы в 1 поток
	queue <- struct{}{}
	defer func() {
		// Запоминаем время последнего запроса и очищаем очередь
		lastTimeRequest = time.Now()

		<-queue
	}()

	reqBody, err := sjson.SetBytes(requestBody, "model", model)
	if err != nil {
		return nil, fmt.Errorf("error in sjson.SetBytes: %w", err)
	}

	if !time.Now().After(lastTimeRequest.Add(maxTimeoutTime)) {
		log.Printf("Throttled %v seс for %s", time.Until(lastTimeRequest.Add(maxTimeoutTime)).Seconds(), model)
		time.Sleep(time.Until(lastTimeRequest.Add(maxTimeoutTime)))
	}

	return gigachatClient.SendBytes(reqBody)
}
