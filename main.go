package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"ai-proxy/internal"

	"gopkg.in/yaml.v3"
)

func handleHTTP(w http.ResponseWriter, req *http.Request) {
	log.Println(req.RemoteAddr, req.Method, req.RequestURI, req.Proto, req.UserAgent())

	startTime := time.Now()

	internal.Req(w, req)

	log.Println("Latency", time.Since(startTime))
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

func main() {
	// Define a flag for the port
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	b, _ := yaml.Marshal(internal.ProvidersMap)

	fmt.Println(string(b))

	log.Printf("Listening on port %d", *port)

	mux := http.NewServeMux()
	// Register the middleware
	// mux.HandleFunc("/", authMiddleware(handleHTTP))
	mux.HandleFunc("/", handleHTTP)
	mux.HandleFunc("/ping", ping)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), mux))
}
