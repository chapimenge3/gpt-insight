package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const openAiUrl = "https://api.openai.com/"

var (
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path"},
	)
	totalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"path"},
	)
)

func openAiProxy(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	start := time.Now() // Start the timer

	url := r.URL.Path
	// strip the /openai prefix
	url = url[7:]
	headers := r.Header
	fmt.Printf(" goturl: %s request\n", url)
	fmt.Printf("headers: %s\n", headers)
	method := r.Method

	req, err := http.NewRequest(method, "https://api.openai.com"+url, nil)
	if err != nil {
		fmt.Printf("Error creating request: %s\n", err)
		log.Fatal(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
		return
	}

	req.Header = headers
	if method == "POST" || method == "PUT" {
		req.Body = r.Body
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error getting response: %s\n", err)
		log.Fatal(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
		return
	}
	defer resp.Body.Close()

	// Calculate latency
	duration := time.Since(start)

	// Record the duration metric
	requestDuration.WithLabelValues(path).Observe(duration.Seconds())

	// Increment the request counter metric
	totalRequests.WithLabelValues(path).Inc()

	fmt.Printf("Request latency: %s\n", duration)

	// Copy headers and status code
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		fmt.Printf("Error copying response body: %s\n", err)
	}
}

func main() {
	prometheus.MustRegister(requestDuration, totalRequests)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /openai/", openAiProxy)
	mux.HandleFunc("POST /openai/", openAiProxy)
	mux.Handle("/metrics", promhttp.Handler())

	fmt.Println("Starting server on port http://localhost:8080/")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		fmt.Printf("Error starting server: %s\n", err)
		log.Fatal(err)
	}
}
