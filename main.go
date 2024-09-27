package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"

	"ai-proxy/internal"

	"gopkg.in/yaml.v3"
)

//go:embed config.yaml
var configBytes []byte

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

func main() {
	// Define a flag for the port
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	var config internal.Config

	err := yaml.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatalf("Error Unmarshal config: %v", err)
	}

	for _, v := range config.Models {
		fmt.Println("Load model ", v.Name)
	}

	log.Printf("Listening on port %d", *port)

	handler, err := internal.NewProxyHandler(config)
	if err != nil {
		log.Fatalf("Error creating proxy handler: %v", err)
	}

	mux := http.NewServeMux()
	// Register the middleware
	// mux.HandleFunc("/", authMiddleware(handleHTTP))
	mux.Handle("/", handler)
	mux.HandleFunc("/ping", ping)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), mux))
}
