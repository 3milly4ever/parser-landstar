package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Define Prometheus metrics
var (
	MessagesReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sqs_worker_messages_received_total",
		Help: "Total number of messages received from SQS",
	})

	MessagesProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sqs_worker_messages_processed_total",
		Help: "Total number of messages successfully processed",
	})

	MessagesFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sqs_worker_messages_failed_total",
		Help: "Total number of messages failed during processing",
	})

	MessagesParsed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sqs_worker_messages_parsed_total",
		Help: "Total number of messages successfully parsed",
	})

	ProcessingDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "sqs_worker_message_processing_duration_seconds",
		Help:    "Duration of message processing in seconds",
		Buckets: prometheus.DefBuckets,
	})
)

// InitializePrometheus initializes the Prometheus metrics and starts the HTTP handler
func InitializePrometheus() {
	// Register metrics with Prometheus
	prometheus.MustRegister(MessagesReceived)
	prometheus.MustRegister(MessagesProcessed)
	prometheus.MustRegister(MessagesFailed)
	prometheus.MustRegister(ProcessingDuration)

	// Start a HTTP server for Prometheus to scrape metrics
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		http.ListenAndServe(":2112", nil) // Expose metrics on :2112/metrics
	}()
}
