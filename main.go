package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"ai-proxy/internal"

	"gopkg.in/yaml.v3"
)

//go:embed config.yaml
var configBytes []byte

// var config internal.Config

type Config struct {
	Models []internal.Model `yaml:"models"`
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer QVBJIFRPS0VOIEZPUiBBSS1QUk9YWQ==" {
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, err := hijacker.Hijack()
				if err == nil {
					conn.Close() // Close the connection to simulate a break.

					return
				}
			}

			http.Error(w, "", http.StatusNetworkAuthenticationRequired)

			return
		}
		// If authorized, call the next handler
		next(w, req)
	}
}

func ping(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("OK"))
}

func listModels(w http.ResponseWriter, req *http.Request) {
	type Data struct {
		ID string `json:"id"`
	}

	type Models struct {
		Object string `json:"object"`
		Data   []Data `json:"data"`
	}

	var models Models

	models.Object = "list"

	for _, v := range internal.Models {
		models.Data = append(models.Data, Data{ID: v.Name})
	}

	models.Data = append(models.Data, Data{ID: "SMALL"})
	models.Data = append(models.Data, Data{ID: "BIG"})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

func main() {
	// Define a flag for the port
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	var config Config

	err := yaml.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatalf("Error Unmarshal config: %v", err)
	}

	internal.RateLimits = make(map[string]*internal.RateLimit)

	for _, v := range config.Models {
		internal.Models = append(internal.Models, v)
		internal.RateLimits[v.Name] = &internal.RateLimit{}

		log.Println("Load model ", v.Name)
	}

	log.Printf("Listening on port %d", *port)

	mux := http.NewServeMux()
	// Register the middleware
	// mux.HandleFunc("/", authMiddleware(handleHTTP))
	mux.HandleFunc("/", internal.HandlerTxt)
	mux.HandleFunc("/image", internal.HandlerImage)
	mux.HandleFunc("/ping", ping)
	mux.HandleFunc("/models", listModels)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), mux))
}
